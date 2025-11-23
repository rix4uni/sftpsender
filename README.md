## SftpSender

A lightweight CLI tool for uploading and downloading files/directories to/from remote servers using SFTP over SSH.

## Features

- 🚀 Fast file and directory transfers using SFTP
- 📁 Support for both single files and entire directories
- 🔐 Secure credential management via YAML configuration
- 🎯 Multiple VPS credential support
- 🏷️ VPS name support - use friendly names instead of remembering IP addresses
- 🏠 Automatic config file setup from GitHub
- 📍 Custom upload/download locations
- 🔕 Silent mode for automation

## Installation

**Using Go:**
```
go install github.com/rix4uni/sftpsender@latest
```

**Pre-built Binaries:**
```
wget https://github.com/rix4uni/sftpsender/releases/download/v0.0.2/sftpsender-linux-amd64-0.0.2.tgz
tar -xvzf sftpsender-linux-amd64-0.0.2.tgz
mv sftpsender ~/go/bin/
```

**From Source:**
```
git clone --depth 1 https://github.com/rix4uni/sftpsender.git
cd sftpsender; go install
```

## Configuration

The tool uses a configuration file located at `~/.config/sftpsender/config.yaml`. On first run, if the file doesn't exist, it will be automatically downloaded from the repository.

### Configuration File Structure

```yaml
default_remote_location: /root

credentials:
  - name: worker1          # Optional: friendly VPS name
    ip: 192.168.1.1
    username: root
    password: yourpassword
    secret: optional_secret_key
  
  - name: worker2          # Optional: friendly VPS name
    ip: 192.168.1.2
    username: admin
    password: anotherpassword
    secret: optional_secret_key
  
  - ip: 192.168.1.3        # Name field is optional - IP-only entries work too
    username: root
    password: anotherpassword
    secret: optional_secret_key
```

**Note:** The `name` field is optional. You can use either IP addresses or VPS names (or both). If a VPS name is provided, you can reference the server using that name instead of the IP address.

### Manual Configuration

You can also manually create or edit the config file:

```yaml
mkdir -p ~/.config/sftpsender
nano ~/.config/sftpsender/config.yaml
```

Or use a custom config file location:

```yaml
sftpsender --upload file.txt --ip 192.168.1.1 --config /path/to/custom/config.yaml
```

## Usage

### Upload Files

Upload a single file using IP address:
```yaml
sftpsender --upload modified-requests2.zip --ip 192.168.1.1
```

Upload a single file using VPS name:
```yaml
sftpsender --upload modified-requests2.zip --ip worker1
```

Upload a directory:
```yaml
sftpsender --upload bbp-scope-moniter --ip worker1
```

Upload to a custom remote location:
```yaml
sftpsender --upload file.txt --ip worker1 --location /custom/path
```

### Download Files

Download a single file using IP address:
```yaml
sftpsender --download modified-requests2.zip --ip 192.168.1.1
```

Download a single file using VPS name:
```yaml
sftpsender --download modified-requests2.zip --ip worker1
```

Download a directory:
```yaml
sftpsender --download bbp-scope-moniter --ip worker1
```

Download to a custom local location:
```yaml
sftpsender --download file.txt --ip worker1 --location /local/path
```

## VPS Name Support

You can use either IP addresses or VPS names with the `--ip` flag:

- **IP Address**: `sftpsender --upload file.txt --ip 192.168.1.1`
- **VPS Name**: `sftpsender --upload file.txt --ip worker1`

The tool will first try to match by VPS name, then fall back to IP address matching if no name match is found. This means:
- If you have a VPS with name "worker1" configured, you can use `--ip worker1`
- If you don't have a name configured, you can still use the IP address directly
- Both methods work seamlessly and maintain full backward compatibility
