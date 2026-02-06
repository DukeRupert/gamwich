# Kiosk Setup Guide

This guide covers setting up Gamwich as a dedicated kiosk on a wall-mounted touchscreen, typically a Raspberry Pi with an attached display.

## Hardware Requirements

- Raspberry Pi 4 (2GB+ RAM) or Pi 5
- Official 7" touchscreen or any HDMI display with USB touchscreen
- MicroSD card (16GB+)
- Power supply (USB-C, 5V/3A for Pi 4)
- Case with VESA mount (optional)
- Ethernet cable or Wi-Fi

## Raspberry Pi OS Setup

1. Flash Raspberry Pi OS Lite (64-bit) using Raspberry Pi Imager.
2. Enable SSH during flashing (set hostname, user, and Wi-Fi if needed).
3. Boot and update:

```bash
sudo apt update && sudo apt upgrade -y
```

4. Install a minimal desktop environment and Chromium:

```bash
sudo apt install -y --no-install-recommends xserver-xorg x11-xserver-utils xinit chromium-browser unclutter
```

## Auto-Login

Configure auto-login to the console (no desktop manager needed):

```bash
sudo raspi-config
# System Options > Boot / Auto Login > Console Autologin
```

## Chromium Kiosk Script

Create `/home/pi/kiosk.sh`:

```bash
#!/bin/bash
xset -dpms       # Disable DPMS (Energy Star) features
xset s off        # Disable screen saver
xset s noblank    # Don't blank the screen

unclutter -idle 0.5 -root &  # Hide mouse cursor after 0.5s

chromium-browser \
  --noerrdialogs \
  --disable-infobars \
  --kiosk \
  --check-for-update-interval=31536000 \
  --disable-pinch \
  --overscroll-history-navigation=0 \
  --disable-features=TranslateUI \
  --autoplay-policy=no-user-gesture-required \
  "http://localhost:8080"
```

```bash
chmod +x /home/pi/kiosk.sh
```

## Systemd Service

Create `/etc/systemd/system/kiosk.service`:

```ini
[Unit]
Description=Gamwich Kiosk
After=network.target gamwich.service

[Service]
User=pi
Environment=DISPLAY=:0
ExecStart=/bin/bash -c 'xinit /home/pi/kiosk.sh -- -nocursor'
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl enable kiosk.service
sudo systemctl start kiosk.service
```

## Gamwich Server Service

Create `/etc/systemd/system/gamwich.service`:

```ini
[Unit]
Description=Gamwich Server
After=network.target

[Service]
User=pi
WorkingDirectory=/opt/gamwich
ExecStart=/opt/gamwich/gamwich
Restart=always
RestartSec=3
Environment=GAMWICH_DB_PATH=/opt/gamwich/data/gamwich.db
Environment=GAMWICH_WEATHER_LAT=47.6062
Environment=GAMWICH_WEATHER_LON=-122.3321
Environment=GAMWICH_WEATHER_UNITS=fahrenheit

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable gamwich.service
sudo systemctl start gamwich.service
```

## Screen Rotation

If your display is mounted in portrait or upside-down, add to `/boot/config.txt`:

```
# Rotate display: 0=normal, 1=90, 2=180, 3=270
display_rotate=0
```

For the official touchscreen, also set touch rotation:

```bash
# In /boot/config.txt
dtoverlay=vc4-kms-v3d
display_auto_detect=1

# For 180-degree rotation of the official display:
lcd_rotate=2
```

## Touchscreen Calibration

If touch input is misaligned:

```bash
sudo apt install -y xinput-calibrator
DISPLAY=:0 xinput_calibrator
```

Follow the on-screen prompts, then copy the output to `/etc/X11/xorg.conf.d/99-calibration.conf`.

## Auto-Restart on Crash

The systemd services above include `Restart=on-failure` / `Restart=always`, which handles process crashes automatically.

For a daily reboot (optional), add a cron job:

```bash
sudo crontab -e
# Add: 0 4 * * * /sbin/reboot
```

## Gamwich Kiosk Settings

Once running, configure kiosk behavior in the Gamwich UI:

- **Settings > Kiosk Mode > Idle Timeout**: Time before the screensaver activates (1-30 minutes).
- **Settings > Kiosk Mode > Quiet Hours**: Dims the display during nighttime hours.
- **Settings > Kiosk Mode > Burn-in Prevention**: Subtle pixel shifting during idle to protect the display.

## Troubleshooting

**Black screen / no display:**
- Check HDMI cable connection
- Try `tvservice -s` to check display status
- Verify `/boot/config.txt` settings

**Touch not working:**
- Check `dmesg | grep -i touch` for USB touch device detection
- Try `DISPLAY=:0 xinput list` to see input devices

**Chromium crashes:**
- Check available memory: `free -h`
- Increase GPU memory in `/boot/config.txt`: `gpu_mem=128`
- Check logs: `journalctl -u kiosk.service`

**Gamwich not loading:**
- Check server logs: `journalctl -u gamwich.service`
- Verify server is running: `curl http://localhost:8080`
- Check database permissions on the data directory
