package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

type CacheTracker struct {
	currentSize     int
	userThreshold   int
	markerThreshold int
	honeycombAPIKey string
	log             logrus.FieldLogger
	lock            sync.Mutex
}

func NewCacheTracker(userThreshold, markerThreshold int, apiKey string, log logrus.FieldLogger) *CacheTracker {
	log.WithFields(logrus.Fields{
		"userThreshold":   userThreshold,
		"markerThreshold": markerThreshold,
	}).Debug("Creating Cache Tracker")

	return &CacheTracker{
		userThreshold:   userThreshold,
		markerThreshold: markerThreshold,
		honeycombAPIKey: apiKey,
		log:             log,
	}
}

func (c *CacheTracker) IsOverUserThreshold() bool {
	return c.currentSize > c.userThreshold
}

func (c *CacheTracker) Track(ctx context.Context, fe *frontendServer) {
	c.log.Infof("starting CacheTracker.Track()")
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			resp, err := fe.getCacheSize(ctx)
			if err != nil {
				c.log.Error(errors.Wrap(err, "could not fetch cache size"))
				continue
			}
			c.updateSize(int(resp.CacheSize))
			c.log.WithFields(logrus.Fields{
				"currentSize": c.currentSize,
			}).Debug("fetched cache size")
		}
	}()
}

func (c *CacheTracker) updateSize(newSize int) {
	c.lock.Lock()

	if newSize > c.markerThreshold && c.currentSize <= c.markerThreshold {
		c.log.WithFields(logrus.Fields{
			"currentSize":     c.currentSize,
			"markerThreshold": c.markerThreshold,
			"newSize":         newSize,
		}).Debug("cache marker threshold reached")

		// Send a marker in a new Go routine
		go c.createMarker()
	}

	c.currentSize = newSize
	c.lock.Unlock()
}

func (c *CacheTracker) createMarker() {

	c.createMarkerHoneycomb()
	MockBuildId = randomHex(4) // update build id

}

func (c *CacheTracker) createMarkerHoneycomb() {
	if c.honeycombAPIKey == "" {
		// Do nothing
		return
	}
	c.log.Debug("Creating Honeycomb marker...")

	url := "https://api.honeycomb.io/1/markers/__all__"
	payload := []byte(`{"message":"Deploy 5645075", "url":"https://github.com/honeycombio/microservices-demo/commit/5645075", "type":"deploy"}`)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(payload))
	if err != nil {
		c.log.Error(errors.Wrap(err, "could not create request to generate marker"))
		return
	}
	req.Header.Set("X-Honeycomb-Team", c.honeycombAPIKey)

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		c.log.Error(errors.Wrap(err, "could not create Honeycomb marker"))
	} else if resp.StatusCode != 200 && resp.StatusCode != 201 {
		c.log.WithFields(logrus.Fields{
			"satusCode": resp.StatusCode,
		}).Error("Invalid status code when creating Honeycomb marker")
		bodyBytes, _ := io.ReadAll(resp.Body)
		c.log.Error(string(bodyBytes))
	} else {
		c.log.Debug("Honeycomb marker created.")
	}
}
