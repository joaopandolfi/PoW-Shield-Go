# PoW-Shield-Go
Proof of work shield written in Golang to protect DDoS atacks

## Description
PoW Shield is a DDoS protection solution for the OSI application layer, functioning as a proxy that employs proof of work to secure the communication between the backend service and the end user. This project offers an alternative to traditional anti-DDoS methods, such as Google's ReCaptcha, which are often cumbersome for users. With PoW Shield, accessing a protected web service is seamless: just navigate to the URL, and your browser will handle the verification process automatically.

## Key Features

- Proof of Work Mechanism: Uses computational challenges to verify legitimate users and deter attackers.
- User-Friendly: Eliminates the need for users to solve complex captchas.
- Seamless Integration: Easily integrates with your existing backend services.
- Web Service Structure
- Proxy Functionality
- PoW Implementation
- WAF Implementation
- Multi-Instance Syncing (Redis)
- SSL Support

## Getting Started
To start using PoW Shield, follow these steps:

- Installation: Clone the repository and follow the setup instructions.
- Configuration: Adjust the settings to fit your backend service requirements.
- Deployment: Deploy the proxy to your desired environment.
- For detailed instructions, please refer to the Installation Guide.

### Backend
1. Configure the env file
2. Copy `.env.example` to `./server/.env`
```sh
cp .env.example server/.env
```

3. Build and execute

```sh
make build-backend
make run-backend
```

#### Check backend vulnerabilities
```sh
make check-vuln
```

### Frontend

```sh
make build-front-tools
make build-front
```

## Simple test
1. Build the backend
2. Build the client
3. Run the server
3. Acess http://localhost:5656/public

### Contributing
We welcome contributions! 

Contact me on joaopandolfi@gmail.com

## License
This project is licensed under the MIT License.
