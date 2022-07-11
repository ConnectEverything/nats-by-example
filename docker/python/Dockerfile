FROM python:3.10.5

RUN pip install nats-py[nkeys]==2.1.4

WORKDIR /opt/app
COPY . ./

CMD ["python", "/opt/app/main.py"]
