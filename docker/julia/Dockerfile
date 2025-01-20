FROM julia:1.10

COPY . .

RUN julia -e 'using Pkg; Pkg.add("NATS"); Pkg.add("JSON3")'

CMD ["julia", "main.jl"]
