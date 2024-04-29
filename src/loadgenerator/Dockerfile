FROM python:3.12-slim as base

FROM base as builder

RUN chmod 1777 /tmp

RUN apt-get -qq update \
    && apt-get install -y --no-install-recommends \
    g++

COPY requirements.txt .

RUN pip install --prefix="/install" -r requirements.txt

FROM base
COPY --from=builder /install /usr/local

COPY . .
RUN chmod +x ./loadgen.sh
RUN apt-get -qq update \
    && apt-get install -y --no-install-recommends \
    curl
ENTRYPOINT ./loadgen.sh
