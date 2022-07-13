FROM python:3.10

RUN useradd --create-home --user-group nats
USER nats

WORKDIR /home/nats/code

COPY requirements.txt ./
RUN pip install -r requirements.txt
COPY . .

CMD ["python", "/home/nats/code/main.py"]
