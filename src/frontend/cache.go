package main

import (
	"bytes"
	"context"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"net/http"
	"sync"
	"time"
)

type CacheTracker struct {
	currentSize      int
	userThreshold    int
	markerThreshold  int
	honeycombAPIKey  string
	honeycombDataset string
	log              logrus.FieldLogger
	lock             sync.Mutex
}

func NewCacheTracker(userThreshold, markerThreshold int, apiKey, dataset string, log logrus.FieldLogger) *CacheTracker {
	return &CacheTracker{
		userThreshold:    userThreshold,
		markerThreshold:  markerThreshold,
		honeycombAPIKey:  apiKey,
		honeycombDataset: dataset,
		log:              log,
	}
}

func (c *CacheTracker) GetSize() int {
	return c.currentSize
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
				return
			}
			c.updateSize(int(resp.CacheSize))
			c.log.Debugf("current cache size is: %d", c.currentSize)
		}
	}()
}

func (c *CacheTracker) updateSize(newSize int) {
	c.lock.Lock()

	if newSize > c.markerThreshold && c.currentSize <= c.markerThreshold {
		c.log.Debugf("marker threshold: %d has been reached, new size: %d", c.markerThreshold, newSize)
		// Send a marker in a new Go routine
		go c.createMarker()
	}

	c.currentSize = newSize
	c.lock.Unlock()
}

func (c *CacheTracker) createMarker() {
	if c.honeycombAPIKey == "" || c.honeycombDataset == "" {
		// Do nothing
		return
	}

	c.log.Debug("Creating Honeycomb marker...")

	url := "https://api.honeycomb.io/1/markers/" + c.honeycombDataset
	payload := []byte(`{"message":"Deploy C34E68A7","type":"deploy"}`)
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
	} else if resp.StatusCode != 200 {
		c.log.Errorf("Invalid status code: %d, when creating Honeycomb marker", resp.StatusCode)
	} else {
		c.log.Debug("Honeycomb marker created.")
	}

}
