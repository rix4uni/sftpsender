## SftpSender

A high-performance CLI tool for uploading and downloading files/directories to/from remote servers using SFTP over SSH. Optimized for speed with concurrent operations and advanced performance features.

## Features

- ⚡ **High-Performance Transfers** - Optimized with concurrent writes/reads and request pipelining
- 📁 Support for both single files and entire directories
- 🔐 Secure credential management via YAML configuration
- 🎯 Multiple VPS credential support
- 🏷️ VPS name support - use friendly names instead of remembering IP addresses
- 🏠 Automatic config file setup from GitHub
- 📍 Custom upload/download locations (integrated into `--ip` flag)
- 🔕 Silent mode for automation
- 📂 Auto-creates remote directories as needed
- 🤖 **Automatic Multi-Worker Distribution** - Automatically send files to multiple workers using ranges or specific numbers with sequential file mapping

## Installation

**Using Go:**
```
go install github.com/rix4uni/sftpsender@latest
```

**Pre-built Binaries:**
```
wget https://github.com/rix4uni/sftpsender/releases/download/v0.0.4/sftpsender-linux-amd64-0.0.4.tgz
tar -xvzf sftpsender-linux-amd64-0.0.4.tgz
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

Upload to a custom remote location (path integrated into `--ip` flag):
```yaml
sftpsender --upload file.txt --ip worker1:/custom/path
```

Upload to a nested directory (auto-creates directories if they don't exist):
```yaml
sftpsender --upload file.txt --ip worker1:this-is-very-long/testx
```

#### Automatic Multi-Worker Upload

Automatically send files to multiple workers using the `--autosend` flag. Files are mapped sequentially to workers, and the tool automatically discovers the next files in sequence.

Send files to specific workers:
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21,27
```
This will send `worker162.txt` to `worker21` and `worker163.txt` to `worker27`.

Send files to a range of workers:
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21-27
```
This will send files sequentially to workers 21 through 27 (7 files total).

Exclude specific workers from a range:
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21-27 --ignore 22,25
```
This will send files to workers 21, 23, 24, 26, 27 (skipping 22 and 25).

**How it works:**
- The `*` wildcard in `--ip *:/path` is replaced with `worker{num}` for each worker
- Files are discovered in sequence: if you specify `worker162.txt`, the tool automatically finds `worker163.txt`, `worker164.txt`, etc.
- Files are mapped sequentially: first file to first worker, second file to second worker, and so on
- The tool validates that all required files exist before starting any uploads
- Progress is shown for each upload with a summary at the end

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

Download from a custom remote location:
```yaml
sftpsender --download file.txt --ip worker1:/remote/path
```

## VPS Name Support

You can use either IP addresses or VPS names with the `--ip` flag:

- **IP Address**: `sftpsender --upload file.txt --ip 192.168.1.1`
- **VPS Name**: `sftpsender --upload file.txt --ip worker1`
- **IP with Path**: `sftpsender --upload file.txt --ip 192.168.1.1:/custom/path`
- **VPS Name with Path**: `sftpsender --upload file.txt --ip worker1:/custom/path`

The tool will first try to match by VPS name, then fall back to IP address matching if no name match is found. This means:
- If you have a VPS with name "worker1" configured, you can use `--ip worker1`
- If you don't have a name configured, you can still use the IP address directly
- Both methods work seamlessly and maintain full backward compatibility

## Path Specification

The `--ip` flag now supports specifying the remote path directly using colon syntax:

- `--ip worker1` - Uses default remote location (from config or `/root`)
- `--ip worker1:/custom/path` - Uploads/downloads to/from `/custom/path`
- `--ip 192.168.1.1:/path/to/file` - Works with IP addresses too

## Autosend Feature

The `--autosend` flag enables automatic file distribution to multiple workers, making it easy to deploy files across your infrastructure.

### How It Works

1. **Worker Selection**: Specify workers using:
   - **Ranges**: `--autosend 21-27` (includes all workers from 21 to 27)
   - **Specific Numbers**: `--autosend 21,27` (only workers 21 and 27)
   - **Combined**: `--autosend 21-27,30,35-40` (range plus specific numbers)

2. **Worker Exclusion**: Use `--ignore` to exclude specific workers from a range:
   - `--autosend 21-27 --ignore 22,25` (sends to 21, 23, 24, 26, 27)

3. **Wildcard Resolution**: The `*` wildcard in `--ip *:/path` is automatically replaced with `worker{num}`:
   - `--ip *:/root/app` becomes `worker21:/root/app`, `worker22:/root/app`, etc.

4. **File Sequence Discovery**: The tool automatically finds files in sequence:
   - If you specify `worker162.txt`, it will find `worker163.txt`, `worker164.txt`, etc.
   - The number in the filename is extracted and incremented for each subsequent file
   - All files must exist before uploads begin

5. **Sequential Mapping**: Files are mapped to workers in order:
   - First file → first worker
   - Second file → second worker
   - And so on...

### Usage Examples

**Send to specific workers:**
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21,27
```
- Sends `worker162.txt` to `worker21`
- Sends `worker163.txt` to `worker27`

**Send to a range with exclusions:**
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21-27 --ignore 22,25
```
- Sends to workers: 21, 23, 24, 26, 27 (5 workers total)
- Requires 5 files: `worker162.txt` through `worker166.txt`

### Progress and Error Handling

- Progress is shown for each upload with relative file paths:
  ```
  [1/5] Uploading to worker21 (worker21)...
  Uploading split/worker162.txt to worker21:/root/react2shell-scanner/worker162.txt
  ✓ Successfully uploaded worker162.txt to worker21
  ```
- File paths are displayed using the same directory structure as your original upload path
- Successful uploads are marked with ✓
- Failed uploads are reported but don't stop other uploads
- A summary is displayed at the end showing success/failure counts
- If any uploads fail, the tool exits with an error code

### Requirements

- `--autosend` can only be used with `--upload` (not with `--download`)
- The number of files found must match the number of workers
- All files in the sequence must exist before uploads begin
- Worker names (e.g., `worker21`) must be configured in your config file

## Performance Optimizations

SftpSender is optimized for high-speed transfers with the following features:

- **Concurrent Operations**: Enabled concurrent writes and reads for up to 64 simultaneous requests per file
- **Request Pipelining**: Multiple SFTP requests can be in flight simultaneously, reducing latency
- **Optimized Buffers**: 256KB buffers (8x the SFTP packet size) for optimal packet alignment
- **TCP Optimizations**: Keepalive and no-delay settings for better network performance
- **Auto-Directory Creation**: Automatically creates remote directories as needed

These optimizations make SftpSender competitive with commercial SFTP clients like Termius.

## Examples

Upload a file to a specific directory (creates directory if needed):
```yaml
sftpsender --upload data.csv --ip worker1:reports/2024/january
```

Upload using IP address with custom path:
```yaml
sftpsender --upload backup.tar.gz --ip 192.168.1.1:/backups
```

Download from a specific remote path:
```yaml
sftpsender --download logs/app.log --ip worker1:/var/log
```

Silent mode for automation (no banner or progress):
```yaml
sftpsender --silent --upload file.txt --ip worker1
```

**Autosend Examples:**

Send files to specific workers (2 files to 2 workers):
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21,27
```
- `worker162.txt` → `worker21`
- `worker163.txt` → `worker27`

Send files to a range of workers with exclusions:
```yaml
sftpsender --upload split/worker162.txt --ip *:/root/react2shell-scanner --autosend 21-27 --ignore 22,25
```
- Sends to workers: 21, 23, 24, 26, 27
- Requires files: `worker162.txt`, `worker163.txt`, `worker164.txt`, `worker165.txt`, `worker166.txt`
- Files are mapped sequentially: worker21 gets worker162.txt, worker23 gets worker163.txt, etc.
