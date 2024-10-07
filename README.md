## Simple Tunnel 🤖 (WIP)

Simple Tunnel is a free, lightweight HTTP tunneling solution written in Go. It allows you to expose your local web server to the internet, making it accessible from anywhere.

## Installation

```
wget https://raw.githubusercontent.com/ghousemohamed/simple-tunnel/main/install.sh -O install.sh && bash install.sh
```

Now you can run `simple-tunnel` from anywhere on your machine!

## Usage

To start the tunnel client, simply run:

```
simple-tunnel --port 3000 --subdomain yoursubdomain
```

Now you can access your app running on port 3000 from https://yoursubdomain.simpletunnel.me

## Self-Hosting Guide

### 1. Compile the simple-tunnel binary

You can compile binary for simple-tunnel by running the following command from the root of the directory:

```
go build .
```

### 2. Systemd configuration

Now you can setup simple-tunnel as a systemd service. An example, systemd configuration file has been provided below.

```
[Unit]
Description=simple-tunnel

[Service]
Type=simple
Restart=always
RestartSec=5s
ExecStart=/root/simple-tunnel/simple-tunnel start

[Install]
WantedBy=multi-user.target
```

The above configuration file can be added here `/lib/systemd/system/simple-tunnel.service`

After the systemd config has been added, run the following commands to start the server:

```
sudo systemctl daemon-reload
sudo systemctl start simple-tunnel
sudo systemctl enable simple-tunnel
```

Check to see if simple-tunnel is running as expected:

```
systemctl status simple-tunnel
```

To tail the logs of the tunnel server you can run the following command:

```
journalctl -u simple-tunnel.service -f
```

### 3. Configure nginx

```
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

upstream tunnel_backend {
    server localhost:8080;
}

server {
    server_name yourdomain.com *.yourdomain.com;

    location / {
        proxy_pass http://tunnel_backend;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection $connection_upgrade;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
        proxy_connect_timeout 300s;
    }

    listen 80;
}
```

## Future work

- [ ] Basic HTTP Authentication
- [ ] Rate limiting
- [ ] Allow only certain routes and methods
- [ ] Tunnel via CONNECT method
- [ ] HTTP/2/3
- [ ] TCP/SSH Tunneling

## Acknowledgements

- This project uses the [Cobra](https://github.com/spf13/cobra) library for building the CLI interface.

## License

MIT