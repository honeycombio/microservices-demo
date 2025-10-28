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

## Docker Compose Quickstart

1. Export your host UID (needed so Airflow can write to mounted volumes) and start
   the stack (Airflow + OpenTelemetry Collector):

   ```bash
   export AIRFLOW_UID=$(id -u)
   docker compose up
   ```

   The image installs dependencies from `requirements.txt` and launches
   `airflow standalone`, which uses the built-in SQLite metadata database.

2. Open `http://localhost:8080/` and log in with the default standalone credentials
   (`admin / admin`). Trigger `process_orders_dag` to process the bundled
   `data/orders.json` file. Invoices will appear under `data/invoices`, and the
   `airflow.cfg` overrides ensure both tracing and metrics are sent to the bundled
   collector (`otel-collector`) listening on `http://otel-collector:4318`.

3. The collector batches spans and relays them to Honeycomb using the API key baked
   into `otel-collector-config.yaml` (`x-honeycomb-team: 4A0OWtbdVS4ArSwzHQJEZA`). Update
   this key if you need to target a different Honeycomb environment.

To stop the demo, use `docker compose down`. Add `--volumes` if you also want to
remove the ephemeral Airflow home volume.
