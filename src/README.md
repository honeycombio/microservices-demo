# Service sources

Each service will have their own readme with further instructions on what the service does, and how it was instrumented using OpenTelemetry.

## Architecture

**Online Boutique** is composed of 10 microservices (plus a load generator) written in 5 different languages that talk to each other over gRPC.

[![Architecture of microservices](../docs/img/architecture-diagram.png)](../docs/img/architecture-diagram.png)

Find **Protocol Buffers Descriptions** at the [`../pb` directory](../pb).

| Service                                          | Language      | Description                                                                                                                       |
|--------------------------------------------------|---------------|-----------------------------------------------------------------------------------------------------------------------------------|
| [adservice](./adservice)                         | Java          | Provides text ads based on given context words.                                                                                   |
| [cartservice](./cartservice)                     | C#            | Stores the items in the user's shopping cart in Redis and retrieves it.                                                           |
| [checkoutservice](./checkoutservice)             | Go            | Retrieves user cart, prepares order and orchestrates the payment, shipping and the email notification.                            |
| [currencyservice](./currencyservice)             | Node.js       | Converts one money amount to another currency. Uses real values fetched from European Central Bank. It's the highest QPS service. |
| [emailservice](./emailservice)                   | Python        | Sends users an order confirmation email (mock).                                                                                   |
| [frontend](./frontend)                           | Go            | Exposes an HTTP server to serve the website. Does not require signup/login and generates session IDs for all users automatically. |
| [loadgenerator](./loadgenerator)                 | Python/Locust | Continuously sends requests imitating realistic user shopping flows to the frontend.                                              |
| [paymentservice](./paymentservice)               | Node.js       | Charges the given credit card info (mock) with the given amount and returns a transaction ID.                                     |
| [productcatalogservice](./productcatalogservice) | Go            | Provides the list of products from a JSON file and ability to search products and get individual products.                        |
| [recommendationservice](./recommendationservice) | Python        | Recommends other products based on what's given in the cart.                                                                      |
| [shippingservice](./shippingservice)             | Go            | Gives shipping cost estimates based on the shopping cart. Ships items to the given address (mock)                                 |
