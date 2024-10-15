FROM mcr.microsoft.com/dotnet/sdk:8.0

WORKDIR /src

COPY . .

RUN dotnet build

CMD ["dotnet", "run", "--no-build"]
