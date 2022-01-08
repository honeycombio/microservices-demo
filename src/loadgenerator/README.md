# loadgenerator

The **loadgenerator** is based on [locust](https://locust.io/), a load testing tool.
Several endpoints are defined in the load testing script as intended targets.
The script contains a list of product IDs which exists in the productcatalog service, including a bad product called `BREAKMENOW` which is intended to generate HTTP 500 errors.
