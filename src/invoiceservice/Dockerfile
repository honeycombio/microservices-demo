FROM apache/airflow:3.1.0-python3.12 AS base

FROM base AS builder

USER root

RUN groupadd --gid 6100 invoicesvc \
    && useradd --uid 6100 --gid 6100 \
    --home /home/invoicesvc --create-home \
    --shell /usr/sbin/nologin --password '*' \
    --system invoicesvc

COPY requirements.txt /tmp/requirements.txt

RUN install -d -m 755 /opt/invoicesvc/site-packages \
    && chown invoicesvc:invoicesvc /opt/invoicesvc/site-packages

USER invoicesvc

RUN python -m pip install --no-cache-dir \
    --target /opt/invoicesvc/site-packages \
    -r /tmp/requirements.txt

USER root

RUN chown -R root:root /opt/invoicesvc/site-packages \
    && chmod -R 755 /opt/invoicesvc/site-packages

FROM base

USER root

RUN groupadd --gid 6100 invoicesvc \
    && useradd --uid 6100 --gid 6100 \
    --home /home/invoicesvc --create-home \
    --shell /usr/sbin/nologin --password '*' \
    --system invoicesvc

COPY --from=builder /opt/invoicesvc/site-packages /usr/local/lib/python3.12/site-packages

RUN mkdir -p /opt/airflow/data/invoices \
    && chmod -R 755 /opt/airflow

COPY dags /opt/airflow/dags
COPY airflow.cfg /opt/airflow/config/airflow.cfg
COPY data/orders.json /opt/airflow/data/orders.json

ENV AIRFLOW_HOME=/opt/airflow \
    AIRFLOW_CONFIG=/opt/airflow/config/airflow.cfg \
    PYTHONUNBUFFERED=1

EXPOSE 8080

ENTRYPOINT [ "bash", "-c", "airflow standalone" ]
