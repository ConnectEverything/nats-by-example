FROM mcr.microsoft.com/dotnet/sdk:10.0

WORKDIR /src

COPY . .

RUN dotnet build

CMD ["dotnet", "run", "--no-build"]
