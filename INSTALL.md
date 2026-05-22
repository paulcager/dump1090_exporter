# Dump1090 Server Build Guide

**Updated for 2025** - Personal build guide for setting up Raspberry Pi ADS-B receivers with dump1090-fa and Prometheus exporters.

## Table of Contents
- [Raspberry Pi Zero/Zero 2 W Setup](#raspberry-pi-zerozero-2-w-setup)
- [Network Configuration](#network-configuration)
- [Serial Console Monitor Setup](#serial-console-monitor-setup)
- [dump1090-fa Installation](#dump1090-fa-installation)
- [Prometheus Exporters](#prometheus-exporters)
- [Prometheus Server Configuration](#prometheus-server-configuration)
- [Docker Alternative](#docker-alternative-recommended)
- [Legacy Guide](#legacy-guide)

---

## Raspberry Pi Zero/Zero 2 W Setup

### Hardware Requirements
- Raspberry Pi Zero W or Zero 2 W
- RTL-SDR dongle (RTL2832U chipset)
- 1090 MHz ADS-B antenna
- MicroSD card (8GB minimum, 16GB+ recommended)
- Stable 5V power supply

### OS Installation (Raspberry Pi Imager - Recommended Method)

1. **Download Raspberry Pi Imager** from https://www.raspberrypi.com/software/

2. **Choose OS**: Raspberry Pi OS Lite (64-bit recommended for Zero 2 W, 32-bit for original Zero W)
   - **Note**: Original Pi Zero W is ARMv6 — use 32-bit OS and ARMv6 binaries for node_exporter etc. Pi Zero 2 W is ARMv7 — use 64-bit OS and ARMv7 binaries.
   - For 2025: Use **Bookworm** (current stable release)
   - For compatibility with older guides: Use **Bullseye** (legacy LTS)

3. **Configure via Imager** (click the gear icon ⚙️):
   - **Set hostname**: Use next number in sequence (e.g., `pi-zero-flights-3`)
   - **Enable SSH**: Check "Enable SSH" with password authentication
   - **Set username/password**: Your standard credentials
   - **Configure WiFi**:
     - SSID: `CAG`
     - Password: Your WiFi password
     - Country: `GB`
   - **Set locale settings**: Timezone `Europe/London`, keyboard `GB`

4. **Write to SD card** and boot the Pi

5. **Find your Pi on the network**:
   ```bash
   # From your computer
   ping pi-zero-flights-3.local
   # Or check router's DHCP client list at 192.168.0.1
   ```

6. **SSH into the Pi**:
   ```bash
   ssh pi-zero-flights-3.local
   # Or use IP directly: ssh 192.168.0.X
   ```

   **Note:** Existing receivers are:
   - `pi-zero-flights` at 192.168.0.25
   - `pi-zero-flights-2` at 192.168.0.27

### Post-Boot Configuration

Add these optimizations to `/boot/firmware/config.txt` (Bookworm) or `/boot/config.txt` (Bullseye):

```bash
sudo nano /boot/firmware/config.txt  # Bookworm
# OR
sudo nano /boot/config.txt           # Bullseye
```

Add the following lines:

```ini
# Reduce GPU memory (headless system doesn't need it)
gpu_mem=16

# Enable UART for debugging (optional)
enable_uart=1

# GPIO shutdown button support (optional - allows safe shutdown via GPIO3)
dtoverlay=gpio-shutdown
```

Save and exit (Ctrl+X, Y, Enter).

### Power Management and LED Control

Add to `/etc/rc.local` (before `exit 0`):

```bash
sudo nano /etc/rc.local
```

Add these lines:

```bash
# Disable activity LED (saves minimal power)
echo none >/sys/class/leds/led0/trigger

# Disable HDMI output (saves ~25mA power)
/usr/bin/tvservice -o

exit 0
```

Make sure the file is executable:

```bash
sudo chmod +x /etc/rc.local
```

### Update System

```bash
sudo apt update
sudo apt upgrade -y
sudo reboot
```

---

## Network Configuration

### DNS Workaround for ISP Router

Our ISP router filters out DNS responses for 192.168.0.0/24 addresses, preventing `.local` hostnames from resolving properly. Work around this by adding static entries to `/etc/hosts`.

**On the Pi:**

```bash
sudo nano /etc/hosts
```

Add these lines:

```
# Main server (Home Assistant, Prometheus, feeders, etc.)
192.168.0.2     pi pi.paulcager.org nextcloud nextcloud.paulcager.org

# ADS-B Receivers
192.168.0.25    pi-zero-flights pi-zero-flights.paulcager.org
192.168.0.27    pi-zero-flights-2 pi-zero-flights-2.paulcager.org
192.168.0.XX    pi-zero-flights-4 pi-zero-flights-4.paulcager.org
# Add future receivers here with next available IPs

# Serial Console Monitors (ESP8266)
# Add monitor IPs as you deploy them

# Other hosts on network
192.168.0.20    paul paul.paulcager.org
192.168.0.12    printer printer.paulcager.org
192.168.0.4     storage storage.paulcager.org
192.168.0.52    towel-rail towel-rail.paulcager.org
192.168.0.13    mac mac.paulcager.org
192.168.0.7     cag7 cag7.paulcager.org
192.168.0.8     cag8 cag8.paulcager.org
192.168.0.9     cag9 cag9.paulcager.org
```

**On your management PC/laptop:**

Add the same entries to your local `/etc/hosts` (Linux/Mac) or `C:\Windows\System32\drivers\etc\hosts` (Windows).

### Assign Static IP (Optional but Recommended)

For Bookworm with NetworkManager:

```bash
# Get connection name
nmcli connection show

# Set static IP (example for next receiver - check which IP is available)
sudo nmcli connection modify "Wired connection 1" ipv4.addresses 192.168.0.XX/24
sudo nmcli connection modify "Wired connection 1" ipv4.gateway 192.168.0.1
sudo nmcli connection modify "Wired connection 1" ipv4.dns "192.168.0.1"
sudo nmcli connection modify "Wired connection 1" ipv4.method manual
sudo nmcli connection up "Wired connection 1"
```

For Bullseye with dhcpcd:

```bash
sudo nano /etc/dhcpcd.conf
```

Add:

```
interface wlan0
static ip_address=192.168.0.XX/24  # Use next available IP
static routers=192.168.0.1
static domain_name_servers=192.168.0.1
```

---

## Serial Console Monitor Setup

For Pis installed in the attic or other hard-to-reach locations, use an ESP8266 running [serial-over-ip](https://github.com/paulcager/serial-over-ip) for remote console access and power management.

### Hardware Setup

**Required:**
- ESP8266 module (ESP-12F or similar)
- Wiring between Pi and ESP8266

**Connections:**
- Pi GPIO 14 (Tx) → ESP8266 Rx
- Pi GPIO 15 (Rx) → ESP8266 Tx
- Pi Run Pin → ESP8266 GPIO 4
- Pi GPIO 3 → ESP8266 GPIO 5
- Common ground

### ESP8266 Configuration

The ESP8266 should be configured with hostname `pi-zero-flights-N-monitor` (where N matches the Pi number).

See [serial-over-ip repository](https://github.com/paulcager/serial-over-ip) for building and flashing firmware.

### Accessing the Serial Console

**Via netcat:**
```bash
nc -v pi-zero-flights-4-monitor 8001
```

**Via screen (better terminal):**
```bash
screen //telnet pi-zero-flights-4-monitor 8001 115200
```

### Remote Power Management

**Safe shutdown:**
```bash
curl http://pi-zero-flights-4-monitor/shutdown
```

**Hard reset:**
```bash
curl http://pi-zero-flights-4-monitor/reset
```

**Wake up:**
```bash
curl http://pi-zero-flights-4-monitor/wake-up
```

---

## dump1090-fa Installation

### Why dump1090-fa?

**dump1090-fa** (FlightAware's fork) is recommended over dump1090-mutability because:
- Actively maintained (2025)
- Better performance and features
- Compatible with all feeders (FR24, FlightAware, RadarBox)
- Official FlightAware support

### Installation Steps

1. **Install RTL-SDR prerequisites**:

```bash
sudo apt install -y rtl-sdr librtlsdr-dev
```

2. **Create udev rule for RTL-SDR dongle**:

```bash
sudo tee /etc/udev/rules.d/10-rtl-sdr.rules >/dev/null <<'EOF'
# RTL-SDR dongle permissions
SUBSYSTEMS=="usb", ATTRS{idVendor}=="0bda", ATTRS{idProduct}=="2838", MODE:="0666"
EOF
```

3. **Blacklist DVB-T drivers** (prevents conflicts):

```bash
sudo tee /etc/modprobe.d/blacklist-rtl2832.conf >/dev/null <<'EOF'
# Blacklist RTL-SDR DVB-T drivers
blacklist rtl2832_sdr
blacklist rtl2832
blacklist dvb_usb_rtl28xxu
EOF
```

4. **Add FlightAware repository**:

```bash
# Download and install repository package
wget https://www.flightaware.com/adsb/piaware/files/packages/pool/piaware/f/flightaware-apt-repository/flightaware-apt-repository_1.3_all.deb
sudo dpkg -i flightaware-apt-repository_1.3_all.deb

# Update package list
sudo apt update
```

5. **Install dump1090-fa**:

```bash
sudo apt install -y dump1090-fa
```

6. **Configure dump1090-fa**:

```bash
sudo dpkg-reconfigure dump1090-fa
```

During configuration:
- **Start automatically?** Yes
- **Receiver location**: Enter your latitude and longitude
- **Enable receiver?** Yes
- **HTTP server port**: 8080 (default)

Alternatively, edit `/etc/default/dump1090-fa` directly:

```bash
sudo nano /etc/default/dump1090-fa
```

Key settings:

```bash
ENABLED=yes
RECEIVER_LAT="your-latitude"    # e.g., 51.5074
RECEIVER_LON="your-longitude"   # e.g., -0.1278
RECEIVER_ALT="your-altitude-meters"  # e.g., 50
```

**Note**: The correct variable names are `RECEIVER_LAT`, `RECEIVER_LON`, and `RECEIVER_ALT` (not `RECEIVER_LATITUDE`, `RECEIVER_LONGITUDE`, or `RECEIVER_ALTITUDE`). Using the wrong names means location is silently omitted from `receiver.json`, causing the exporter to produce no aircraft count or distance metrics.

7. **Reboot to apply udev rules and start dump1090**:

```bash
sudo reboot
```

8. **Verify dump1090 is running**:

```bash
sudo systemctl status dump1090-fa

# Check web interface (from your computer browser)
# http://adsb-receiver.local:8080
```

You should see aircraft on the map if everything is working!

---

## Prometheus Exporters

### Node Exporter (System Metrics)

**Node Exporter** provides system metrics (CPU, memory, disk, network) to Prometheus.

#### Installation from Binary (Recommended)

1. **Download the latest ARM binary**:

```bash
# For Pi Zero W (ARMv6):
cd /tmp
wget https://github.com/prometheus/node_exporter/releases/download/v1.8.2/node_exporter-1.8.2.linux-armv6.tar.gz
tar xvfz node_exporter-1.8.2.linux-armv6.tar.gz

# For Pi Zero 2 W (ARMv7):
# wget https://github.com/prometheus/node_exporter/releases/download/v1.8.2/node_exporter-1.8.2.linux-armv7.tar.gz
# tar xvfz node_exporter-1.8.2.linux-armv7.tar.gz
```

Check https://github.com/prometheus/node_exporter/releases for the latest version.

2. **Install binary**:

```bash
sudo cp node_exporter-*/node_exporter /usr/local/bin/
sudo chown root:root /usr/local/bin/node_exporter
```

3. **Create prometheus user**:

```bash
sudo useradd --no-create-home --shell /bin/false prometheus
```

4. **Create systemd service**:

```bash
sudo tee /etc/systemd/system/prometheus-node-exporter.service >/dev/null <<'EOF'
[Unit]
Description=Prometheus Node Exporter
Documentation=https://github.com/prometheus/node_exporter
After=network-online.target

[Service]
Type=simple
User=prometheus
Group=prometheus
Restart=on-failure
ExecStart=/usr/local/bin/node_exporter \
  --no-collector.rapl \
  --no-collector.pressure \
  --no-collector.systemd \
  --no-collector.arp \
  --no-collector.nfs \
  --no-collector.nfsd

[Install]
WantedBy=multi-user.target
EOF
```

5. **Enable and start**:

```bash
sudo systemctl daemon-reload
sudo systemctl enable prometheus-node-exporter
sudo systemctl start prometheus-node-exporter
sudo systemctl status prometheus-node-exporter
```

6. **Test metrics**:

```bash
curl http://localhost:9100/metrics
```

### dump1090_exporter (ADS-B Metrics)

**dump1090_exporter** provides ADS-B receiver metrics to Prometheus.

#### Installation from Binary

1. **Download or build dump1090_exporter**:

```bash
# Option A: Download pre-built binary from GitHub releases
# (Check https://github.com/paulcager/dump1090_exporter/releases)

# Option B: Build from source (requires Go)
cd /tmp
git clone https://github.com/paulcager/dump1090_exporter.git
cd dump1090_exporter
go build -o dump1090_exporter
```

2. **Install binary**:

```bash
sudo cp dump1090_exporter /usr/local/bin/
sudo chown root:root /usr/local/bin/dump1090_exporter
```

3. **Create systemd service**:

```bash
sudo tee /etc/systemd/system/prometheus-dump1090-exporter.service >/dev/null <<'EOF'
[Unit]
Description=Prometheus exporter for dump1090
Documentation=https://github.com/paulcager/dump1090_exporter
After=network-online.target dump1090-fa.service

[Service]
Type=simple
User=prometheus
Group=prometheus
Restart=always
ExecStart=/usr/local/bin/dump1090_exporter \
  --dump1090.files=/run/dump1090-fa/%%s \
  --web.disable-exporter-metrics
ExecReload=/bin/kill -HUP $MAINPID
TimeoutStopSec=20s
SendSIGKILL=no

[Install]
WantedBy=multi-user.target
EOF
```

**Note**: Use `--dump1090.files` with the path to the JSON files:
- **dump1090-fa**: `--dump1090.files=/run/dump1090-fa/%%s`
- **dump1090-mutability**: `--dump1090.files=/run/dump1090-mutability/%%s`

The `%%s` double-percent is required to escape the `%s` placeholder from systemd's unit file specifier expansion. Using `%s` directly will cause systemd to expand it incorrectly.

The `--dump1090.address=http://localhost:8080` HTTP mode does not work with dump1090-fa as it returns HTML rather than JSON at that path.

4. **Enable and start**:

```bash
sudo systemctl daemon-reload
sudo systemctl enable prometheus-dump1090-exporter
sudo systemctl start prometheus-dump1090-exporter
sudo systemctl status prometheus-dump1090-exporter
```

5. **Test metrics**:

```bash
curl http://localhost:9799/metrics
```

You should see metrics like:
- `dump1090_aircraft_count`
- `dump1090_aircraft_messages`
- `dump1090_aircraft_max_distance`

---

## Docker Alternative (Recommended)

For a more modern deployment, consider using Docker containers from this project suite:

### Prerequisites

```bash
# Install Docker
curl -fsSL https://get.docker.com -o get-docker.sh
sudo sh get-docker.sh
sudo usermod -aG docker $USER
```

Log out and back in for group changes to take effect.

### Using dump1090_exporter Container

```bash
docker run -d \
  --name dump1090_exporter \
  --restart unless-stopped \
  -p 9799:9799 \
  ghcr.io/paulcager/dump1090_exporter:latest \
  --dump1090.address=http://host.docker.internal:8080 \
  --web.disable-exporter-metrics
```

For more details, see the main [CLAUDE.md](/home/paul/git/1090/CLAUDE.md) and individual component documentation.

---

## Prometheus Server Configuration

### Adding New Receivers to Prometheus

When you add a new Pi receiver, you need to update the Prometheus configuration on the main server (`pi` at 192.168.0.2).

**Setup Note:** The main server runs Prometheus in Docker (see [ha-caddy](../../ha-caddy/) repository for full infrastructure).

**Location:** `~/git/ha-caddy/prometheus/prometheus.yml` (on `pi`)

Add scrape targets for both the system metrics (node_exporter) and ADS-B metrics (dump1090_exporter):

```yaml
scrape_configs:
  # ADS-B metrics (dump1090_exporter on port 9799)
  - job_name: dump1090
    static_configs:
      - targets:
        - pi-zero-flights:9799
        - pi-zero-flights-2:9799
        # Add new receiver here

  # System metrics (node_exporter on port 9100)
  - job_name: node
    metric_relabel_configs:
      # ... existing relabel configs ...
    static_configs:
      - targets:
         - pi:9100
         - pi-zero-flights:9100
         - pi-zero-flights-2:9100
         # Add new receiver here
         - cag7:9100
         - cag8:9100
         - cag9:9100
         - paul:9100
         - mac:9100
         - storage:9100
         # ... other hosts ...
```

### Docker extra_hosts (Required for hostname resolution)

Prometheus runs inside a Docker container and cannot use the host's `/etc/hosts` directly. Because the ISP router filters DNS responses for local IPs, you must also add new receivers to the `extra_hosts` section of the Prometheus service in `~/git/ha-caddy/docker-compose.yml`:

```yaml
services:
  prometheus:
    extra_hosts:
      - "pi-zero-flights:192.168.0.25"
      - "pi-zero-flights-2:192.168.0.27"
      - "pi-zero-flights-4:192.168.0.XX"  # Add new receiver here
```

Without this, Prometheus will fail to resolve the hostname and the scrape targets will show as DOWN even though `prometheus.yml` is correct.

**After editing, reload Prometheus:**

```bash
# On the main server (pi)
cd ~/git/ha-caddy

# Restart the Prometheus container
docker-compose restart prometheus

# Check logs
docker-compose logs -f prometheus
```

**Verify targets are up:**

Visit `http://pi:9090/targets` (or `http://192.168.0.2:9090/targets`) and verify:
- New receiver at `:9100` shows as UP (node_exporter)
- New receiver at `:9799` shows as UP (dump1090_exporter)

### Grafana Dashboard Updates

If you're using Grafana dashboards that filter by instance, make sure to update dashboard variables or filters to include the new receiver.

### Quick Health Check

Test that metrics are accessible from main server:

```bash
# From pi (192.168.0.2), test connectivity to new receiver
curl http://pi-zero-flights-2:9100/metrics | head
curl http://pi-zero-flights-2:9799/metrics | head
```

---

## Troubleshooting

### dump1090-fa not receiving data

```bash
# Check USB device is detected
lsusb | grep Realtek

# Test RTL-SDR directly
rtl_test

# Check dump1090-fa logs
sudo journalctl -u dump1090-fa -f

# Verify antenna connection and placement
```

### Exporters not working

```bash
# Check service status
sudo systemctl status prometheus-node-exporter
sudo systemctl status prometheus-dump1090-exporter

# Check logs
sudo journalctl -u prometheus-node-exporter -f
sudo journalctl -u prometheus-dump1090-exporter -f

# Verify ports are listening
sudo netstat -tlnp | grep -E '9100|9799'
```

### WiFi connectivity issues

**Bookworm (NetworkManager)**:
```bash
# Check WiFi status
nmcli device status
nmcli connection show

# Reconnect to WiFi
sudo nmcli device wifi connect "SSID" password "PASSWORD"
```

**Bullseye (wpa_supplicant)**:
```bash
# Check WiFi status
sudo wpa_cli status

# Restart WiFi
sudo systemctl restart wpa_supplicant
```

---

## Performance Tips

### Optimize for Pi Zero

The Pi Zero has limited resources. Consider:

1. **Use Lite OS** (no desktop environment)
2. **Disable unnecessary services**:
   ```bash
   sudo systemctl disable bluetooth
   sudo systemctl disable avahi-daemon
   ```
3. **Reduce logging** (optional):
   ```bash
   sudo journalctl --vacuum-time=7d
   ```

### Monitoring Performance

```bash
# CPU temperature
vcgencmd measure_temp

# Memory usage
free -h

# CPU usage
top
```

---

## Next Steps

- **Set up feeders**: See [fr24feeder-container](../fr24feeder-container/CLAUDE.md), [flightaware-container](../flightaware-container/CLAUDE.md), [rbfeeder-container](../rbfeeder-container/CLAUDE.md)
- **Aggregation**: Deploy [dump1090-proxy](../dump1090-proxy/CLAUDE.md) for multi-receiver setups
- **Visualization**: Set up Prometheus and Grafana dashboards
- **Coverage analysis**: Use [utils1090](../utils1090/CLAUDE.md) for coverage mapping

---

## Legacy Guide

The original guide (for Bullseye with dump1090-mutability) is preserved below for reference:

<details>
<summary>Click to expand legacy guide</summary>

### Pi Zero Install (Legacy)

* Write "Lite" image to SD card.
* Assuming SD card's /boot is mounted as /tmp/sdc1:
  * `touch /tmp/sdc1/ssh`
  * Write the following to `/tmp/sdc1/wpa_supplicant.conf`:
    ```
    country=GB
    ctrl_interface=DIR=/var/run/wpa_supplicant GROUP=netdev
    update_config=1

    network={
    scan_ssid=1
    ssid="YOURSSID"
    psk="yourpassword"
    }
    ```
* Unmount SD card, put it in the Pi Zero, boot and do a normal installation.
* Add to `/boot/config.txt`:
  ```
  gpu_mem=16
  enable_uart=1
  dtoverlay=gpio-shutdown
  ```

### Dump1090 Installation (Legacy)

* Apt update and upgrade.
* `apt install dump1090-mutability`
* `echo 'SUBSYSTEMS=="usb", ATTRS{idVendor}=="0bda", ATTRS{idProduct}=="2838", MODE:="0666"' >/etc/udev/rules.d/10-rtl-sdr.rules`
* Blacklist modules in `/etc/modprobe.d/blacklist-rtl2832.conf`:
  * rtl2832_sdr
  * rtl2832
  * dvb_usb_rtl28xxu
* Set START_DUMP1090, Lat/Lon in `/etc/default/dump1090-mutability`.
* Add to `/etc/rc.local`:
  ```
  echo none >/sys/class/leds/led0/trigger
  tvservice -o
  ```
* Reboot to pick up rtlsdr udev rules and modules.

### Node_exporter and dump1090_exporter (Legacy)

* `apt install node_exporter` (if available in repos).
  * Edit `/etc/default/prometheus-node-exporter`:
    ```
    ARGS="--no-collector.rapl --no-collector.pressure --no-collector.systemd --no-collector.arp --no-collector.nfs --no-collector.nfsd"
    ```
* Install `/usr/local/bin/dump1090_exporter`
* Install `/lib/systemd/system/prometheus-dump1090-exporter.service`.

</details>

---

## Quick Reference

### Hostnames and IPs
- **Main Server**: `pi` (192.168.0.2) - Prometheus, Home Assistant, feeders, dump1090-proxy
- **Receivers**:
  - `pi-zero-flights` (192.168.0.25)
  - `pi-zero-flights-2` (192.168.0.27)
  - Add new receivers with next available IPs
- **Monitors**: `pi-zero-flights-N-monitor` (ESP8266 for serial console)

### Ports
- **8080**: dump1090-fa web interface
- **9100**: node_exporter (system metrics)
- **9799**: dump1090_exporter (ADS-B metrics)
- **8001**: Serial console (on monitor ESP8266)
- **30005**: BEAST protocol output from dump1090

### Common Commands

**Access serial console:**
```bash
nc -v pi-zero-flights-2-monitor 8001
```

**Check dump1090 status:**
```bash
sudo systemctl status dump1090-fa
```

**Check exporters:**
```bash
sudo systemctl status prometheus-node-exporter
sudo systemctl status prometheus-dump1090-exporter
```

**View metrics:**
```bash
curl http://pi-zero-flights-2:9100/metrics | grep -v "^#"
curl http://pi-zero-flights-2:9799/metrics | grep -v "^#"
```

**Remote power management:**
```bash
curl http://pi-zero-flights-2-monitor/shutdown  # Safe shutdown
curl http://pi-zero-flights-2-monitor/reset     # Hard reset
curl http://pi-zero-flights-2-monitor/wake-up   # Wake up
```

**View Prometheus targets:**
```bash
# From your browser
http://pi:9090/targets

# From command line
curl http://pi:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.instance | contains("pi-zero")) | {instance: .labels.instance, health: .health}'
```
