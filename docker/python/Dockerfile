FROM python:3.14-slim

COPY requirements.txt ./
RUN pip install -r requirements.txt
COPY . .

CMD ["python", "main.py"]
