FROM grpc/python:1.0

# show python logs as they occur
ENV PYTHONUNBUFFERED=0

# add files into working directory
ADD ./*.py /home/
WORKDIR /home

# set listen port
ENV PORT=8080

ENTRYPOINT python /home/recommendation_server.py $PORT