FROM python:3.15-slim

COPY requirements.txt ./
RUN pip install -r requirements.txt
COPY . .

CMD ["python", "main.py"]
