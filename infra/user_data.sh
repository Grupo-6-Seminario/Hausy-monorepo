#!/bin/bash
# First boot only: Node, Caddy and the three services. scripts/deploy.sh puts each release in
# /opt/hausy/releases, points /opt/hausy/current at it and writes /etc/hausy/*.env, so the
# hausy units fail until the first deploy.
set -euo pipefail

dnf install -y jq tar xz gzip
cd /tmp
curl -fsSLo node.tar.xz https://nodejs.org/dist/v22.23.4/node-v22.23.4-linux-arm64.tar.xz
echo "6287aa46b6c375d816e63ae66d024fbbfd70404825aabdfdf898a970871d2f4b  node.tar.xz" | sha256sum -c
tar -xJf node.tar.xz -C /usr/local --strip-components=1 --no-same-owner
curl -fsSLo caddy.tar.gz https://github.com/caddyserver/caddy/releases/download/v2.11.7/caddy_2.11.7_linux_arm64.tar.gz
echo "3db36ba90c7a6e8dda40ee3dd71fa08844c76b5fb08f61b31e5e78d2ed38e71c51dc7baed875e50d1ca1279196e84302967237386ae87c91ae9f2aaceada682e  caddy.tar.gz" | sha512sum -c
tar -xzf caddy.tar.gz -C /usr/local/bin caddy
mkdir -p /opt/hausy/releases /etc/hausy /etc/caddy

# flush_interval -1 passes the agent's NDJSON stream through unbuffered.
cat > /etc/caddy/Caddyfile <<'EOF'
${site_host} {
	reverse_proxy 127.0.0.1:3000 {
		flush_interval -1
	}
}
EOF

cat > /etc/systemd/system/caddy.service <<'EOF'
[Unit]
Description=Caddy
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=/usr/local/bin/caddy run --config /etc/caddy/Caddyfile
DynamicUser=yes
StateDirectory=caddy
Environment=XDG_DATA_HOME=/var/lib XDG_CONFIG_HOME=/var/lib
AmbientCapabilities=CAP_NET_BIND_SERVICE
Restart=always

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/hausy-api.service <<'EOF'
[Unit]
Description=Hausy API
Wants=network-online.target
After=network-online.target

[Service]
EnvironmentFile=/etc/hausy/api.env
ExecStart=/opt/hausy/current/bin/hausy
DynamicUser=yes
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

cat > /etc/systemd/system/hausy-web.service <<'EOF'
[Unit]
Description=Hausy web
After=hausy-api.service

[Service]
EnvironmentFile=/etc/hausy/web.env
ExecStart=/usr/local/bin/node /opt/hausy/current/web/server.js
DynamicUser=yes
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable hausy-api hausy-web
systemctl enable --now caddy
