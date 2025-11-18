# Airflow Order Processing DAG

This directory contains everything required to schedule the `process_orders_dag`
inside Apache Airflow 3.1.0 (Python 3.12 base image). The workflow reads the
synthetic orders payload mounted at `/opt/airflow/data/orders.json`, generates
invoice artefacts for each entry, and relies on Airflow's built-in OpenTelemetry
integration for tracing.

## Layout

- `dags/process_orders_dag.py` – main DAG definition that reads orders and creates invoices.
- `data/orders.json` – source orders payload copied from the invoices service.
- `data/invoices/` – output directory for generated invoices (populated at runtime).
- `requirements.txt` – Python dependencies to install within your Airflow image.
- `otel-collector-config.yaml` – OpenTelemetry Collector config that batches spans and exports them to Honeycomb.
- `airflow.cfg` – overrides enabling Airflow OpenTelemetry traces and metrics to point at the bundled collector.

## Publish to Amazon ECR

The GitHub Actions workflow in `.github/workflows/invoiceservice.yml` builds this image and pushes it to Amazon ECR
whenever changes land on `main` under `src/invoiceservice/**`. It tags the image as both `latest` and the commit SHA and
pushes to `${{ steps.login-ecr.outputs.registry }}/microservices-demo/invoiceservice`.


## Run locally with Skaffold

You can use the root `skaffold.yaml` to build and deploy the invoiceservice (and the rest of Online Boutique) into your
cluster:

```bash
# Optional: override the destination registry for built images
export SKAFFOLD_DEFAULT_REPO=<your-registry>

skaffold dev
# or, for a one-off build/deploy
skaffold run
```

Skaffold watches for changes under `src/invoiceservice` and rebuilds only the affected image. Ensure your cluster has the
required secrets (e.g., Honeycomb API key) before running.
