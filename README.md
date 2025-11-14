## Sender

A lightweight CLI tool for uploading and downloading files/directories to/from remote servers using SFTP over SSH.

## Features

- 🚀 Fast file and directory transfers using SFTP
- 📁 Support for both single files and entire directories
- 🔐 Secure credential management via YAML configuration
- 🎯 Multiple VPS credential support
- 🏠 Automatic config file setup from GitHub
- 📍 Custom upload/download locations
- 🔕 Silent mode for automation

## Installation

**Using Go:**
```
go install github.com/rix4uni/sender@latest
```

**Pre-built Binaries:**
```
wget https://github.com/rix4uni/sender/releases/download/v0.0.1/sender-linux-amd64-0.0.1.tgz
tar -xvzf sender-linux-amd64-0.0.1.tgz
mv sender ~/go/bin/
```

**From Source:**
```
git clone --depth 1 https://github.com/rix4uni/sender.git
cd sender; go install
```

## Configuration

The tool uses a configuration file located at `~/.config/sender/config.yaml`. On first run, if the file doesn't exist, it will be automatically downloaded from the repository.

### Configuration File Structure

```yaml
default_remote_location: /root

credentials:
  - ip: 192.168.1.1
    username: root
    password: yourpassword
    secret: optional_secret_key
  
  - ip: 192.168.1.2
    username: admin
    password: anotherpassword
    secret: optional_secret_key
```

### Manual Configuration

You can also manually create or edit the config file:

```yaml
mkdir -p ~/.config/sender
nano ~/.config/sender/config.yaml
```

Or use a custom config file location:

```yaml
sender --upload file.txt --ip 192.168.1.1 --config /path/to/custom/config.yaml
```

## Usage

### Upload Files

Upload a single file:
```yaml
sender --upload modified-requests2.zip --ip 192.168.1.1
```

Upload a directory:
```yaml
sender --upload bbp-scope-moniter --ip 192.168.1.1
```

Upload to a custom remote location:
```yaml
sender --upload file.txt --ip 192.168.1.1 --location /custom/path
```

### Download Files

Download a single file:
```yaml
sender --download modified-requests2.zip --ip 192.168.1.1
```

Download a directory:
```yaml
sender --download bbp-scope-moniter --ip 192.168.1.1
```

Download to a custom local location:
```yaml
sender --download file.txt --ip 192.168.1.1 --location /local/path
```
