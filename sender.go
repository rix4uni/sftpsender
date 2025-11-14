package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"

	"github.com/rix4uni/sender/banner"
)

type Config struct {
	Credentials           []Credential `yaml:"credentials"`
	DefaultRemoteLocation string       `yaml:"default_remote_location"`
}

type Credential struct {
	IP       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Secret   string `yaml:"secret"`
}

type Sender struct {
	config *Config
}

func expandHomeDir(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(homeDir, path[2:])
	}
	return path
}

func ensureConfigExists(configPath string) error {
	// Expand home directory if needed
	configPath = expandHomeDir(configPath)

	// Check if config file exists
	if _, err := os.Stat(configPath); err == nil {
		return nil // File exists
	}

	// Create directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	// Download config file
	fmt.Printf("Downloading config file to %s...\n", configPath)
	configURL := "https://raw.githubusercontent.com/rix4uni/sender/refs/heads/main/config.yaml"

	resp, err := http.Get(configURL)
	if err != nil {
		return fmt.Errorf("failed to download config file: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download config file: HTTP %d", resp.StatusCode)
	}

	// Create config file
	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer configFile.Close()

	// Copy content
	if _, err := io.Copy(configFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write config file: %v", err)
	}

	fmt.Println("Config file downloaded successfully!")
	return nil
}

func NewSender(configPath string) (*Sender, error) {
	config := &Config{}

	// Expand home directory
	configPath = expandHomeDir(configPath)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	if err := yaml.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %v", err)
	}

	if config.DefaultRemoteLocation == "" {
		config.DefaultRemoteLocation = "/root"
	}

	return &Sender{config: config}, nil
}

func (s *Sender) findCredential(ip string) (*Credential, error) {
	for _, cred := range s.config.Credentials {
		if cred.IP == ip {
			return &cred, nil
		}
	}
	return nil, fmt.Errorf("no credentials found for IP: %s", ip)
}

func (s *Sender) Upload(localPath, ip, remoteLocation string) error {
	cred, err := s.findCredential(ip)
	if err != nil {
		return err
	}

	if remoteLocation == "" {
		remoteLocation = s.config.DefaultRemoteLocation
	}

	// Get just the filename/dirname for remote path
	baseName := filepath.Base(localPath)
	remotePath := fmt.Sprintf("%s/%s", strings.TrimSuffix(remoteLocation, "/"), baseName)

	fmt.Printf("Uploading %s to %s:%s\n", localPath, ip, remotePath)

	// Check if local path is directory
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat local path: %v", err)
	}

	client, err := s.getSSHClient(cred)
	if err != nil {
		return err
	}
	defer client.Close()

	if info.IsDir() {
		return s.uploadDirectorySFTP(client, localPath, remotePath)
	}
	return s.uploadFileSFTP(client, localPath, remotePath)
}

func (s *Sender) Download(remotePath, ip, localLocation string) error {
	cred, err := s.findCredential(ip)
	if err != nil {
		return err
	}

	if localLocation == "" {
		localLocation = "."
	}

	// Get just the filename/dirname for local path
	baseName := filepath.Base(remotePath)
	localPath := filepath.Join(localLocation, baseName)

	fmt.Printf("Downloading %s:%s to %s\n", ip, remotePath, localPath)

	client, err := s.getSSHClient(cred)
	if err != nil {
		return err
	}
	defer client.Close()

	// Use SFTP to check if it's a directory and download accordingly
	return s.downloadSFTP(client, remotePath, localPath)
}

// SFTP-based implementations
func (s *Sender) uploadFileSFTP(client *ssh.Client, localPath, remotePath string) error {
	sftpClient, err := s.getSFTPClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %v", err)
	}
	defer localFile.Close()

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %v", err)
	}
	defer remoteFile.Close()

	// Copy content
	_, err = io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

func (s *Sender) uploadDirectorySFTP(client *ssh.Client, localPath, remotePath string) error {
	sftpClient, err := s.getSFTPClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Create remote directory
	err = sftpClient.MkdirAll(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote directory: %v", err)
	}

	return filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			return err
		}

		remoteFilePath := filepath.Join(remotePath, relPath)

		if info.IsDir() {
			return sftpClient.MkdirAll(remoteFilePath)
		}

		return s.uploadFileSFTP(client, path, remoteFilePath)
	})
}

func (s *Sender) downloadSFTP(client *ssh.Client, remotePath, localPath string) error {
	sftpClient, err := s.getSFTPClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Check if remote path is file or directory
	remoteInfo, err := sftpClient.Stat(remotePath)
	if err != nil {
		return fmt.Errorf("failed to stat remote path: %v", err)
	}

	if remoteInfo.IsDir() {
		return s.downloadDirectorySFTP(sftpClient, remotePath, localPath)
	}
	return s.downloadFileSFTP(sftpClient, remotePath, localPath)
}

func (s *Sender) downloadFileSFTP(sftpClient *sftp.Client, remotePath, localPath string) error {
	// Create local directory if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Open remote file
	remoteFile, err := sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %v", err)
	}
	defer remoteFile.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %v", err)
	}
	defer localFile.Close()

	// Copy content
	_, err = io.Copy(localFile, remoteFile)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

func (s *Sender) downloadDirectorySFTP(sftpClient *sftp.Client, remotePath, localPath string) error {
	// Create local directory
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %v", err)
	}

	// Walk remote directory
	walker := sftpClient.Walk(remotePath)
	for walker.Step() {
		if err := walker.Err(); err != nil {
			return err
		}

		relPath, err := filepath.Rel(remotePath, walker.Path())
		if err != nil {
			return err
		}

		localFilePath := filepath.Join(localPath, relPath)

		if walker.Stat().IsDir() {
			if err := os.MkdirAll(localFilePath, 0755); err != nil {
				return err
			}
		} else {
			if err := s.downloadFileSFTP(sftpClient, walker.Path(), localFilePath); err != nil {
				return err
			}
		}
	}

	return nil
}

// SSH and SFTP client helpers
func (s *Sender) getSSHClient(cred *Credential) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: cred.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cred.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return ssh.Dial("tcp", fmt.Sprintf("%s:22", cred.IP), config)
}

func (s *Sender) getSFTPClient(sshClient *ssh.Client) (*sftp.Client, error) {
	return sftp.NewClient(sshClient)
}

func main() {
	var (
		upload     = pflag.String("upload", "", "Local file/directory to upload")
		download   = pflag.String("download", "", "Remote file/directory to download")
		ip         = pflag.String("ip", "", "VPS IP address (required)")
		location   = pflag.String("location", "", "Custom remote location for upload or local location for download")
		configPath = pflag.String("config", "~/.config/sender/config.yaml", "Path to config file")
		silent     = pflag.Bool("silent", false, "Silent mode.")
		version    = pflag.Bool("version", false, "Print the version of the tool and exit.")
	)

	pflag.Parse()

	// Print version and exit if -version flag is provided
	if *version {
		banner.PrintBanner()
		banner.PrintVersion()
		return
	}

	// Don't Print banner if -silnet flag is provided
	if !*silent {
		banner.PrintBanner()
	}

	if *ip == "" {
		log.Fatal("IP address is required. Use --ip flag")
	}

	if (*upload == "" && *download == "") || (*upload != "" && *download != "") {
		log.Fatal("You must specify either --upload or --download (but not both)")
	}

	// Ensure config file exists
	if err := ensureConfigExists(*configPath); err != nil {
		log.Fatalf("Failed to ensure config file exists: %v", err)
	}

	sender, err := NewSender(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize sender: %v", err)
	}

	if *upload != "" {
		if err := sender.Upload(*upload, *ip, *location); err != nil {
			log.Fatalf("Upload failed: %v", err)
		}
		fmt.Println("Upload completed successfully!")
	} else if *download != "" {
		if err := sender.Download(*download, *ip, *location); err != nil {
			log.Fatalf("Download failed: %v", err)
		}
		fmt.Println("Download completed successfully!")
	}
}
