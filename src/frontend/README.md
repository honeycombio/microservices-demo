# Frontend

The **Frontend** service is responsible for rendering the UI for the store's website.
It serves as the main entry point for the application.
The application uses Server Side Rendering (SSR) to generate HTML consumed by the browser.

The following routes are defined by the frontend:

| Path              | Method | Use                               |
|-------------------|--------|-----------------------------------|
| `/`               | GET    | Main index page                   |
| `/cart`           | GET    | View Cart                         |
| `/cart`           | POST   | Add to Cart                       |
| `/cart/checktout` | POST   | Place Order                       |
| `/cart/empty`     | POST   | Empty Cart                        |
| `/logout`         | GET    | Logout                            |
| `/product/{id}`   | GET    | View Product                      |
| `/setCurrency`    | POST   | Set Currency                      |
| `/static/`        | *      | Static resources                  |
| `/dist/`          | *      | Compiled Javascript resources     |
| `/robots.txt`     | *      | Search engine response (disallow) |
| `/_healthz`       | *      | Health check (ok)                 |
