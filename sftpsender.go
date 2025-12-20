package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"github.com/spf13/pflag"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"

	"github.com/rix4uni/sftpsender/banner"
)

type Config struct {
	Credentials           []Credential `yaml:"credentials"`
	DefaultRemoteLocation string       `yaml:"default_remote_location"`
}

type Credential struct {
	Name     string `yaml:"name"`
	IP       string `yaml:"ip"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Secret   string `yaml:"secret"`
}

type SftpSender struct {
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
	configURL := "https://raw.githubusercontent.com/rix4uni/sftpsender/refs/heads/main/config.yaml"

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

func NewSftpSender(configPath string) (*SftpSender, error) {
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

	return &SftpSender{config: config}, nil
}

func (s *SftpSender) findCredential(ip string) (*Credential, error) {
	// First, try to match by VPS name
	for _, cred := range s.config.Credentials {
		if cred.Name != "" && cred.Name == ip {
			return &cred, nil
		}
	}
	// If no name match found, fall back to IP matching (backward compatibility)
	for _, cred := range s.config.Credentials {
		if cred.IP == ip {
			return &cred, nil
		}
	}
	return nil, fmt.Errorf("no credentials found for IP or VPS name: %s", ip)
}

func (s *SftpSender) Upload(localPath, ip, remoteLocation string, displayPath ...string) error {
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

	// Use displayPath if provided, otherwise use localPath
	pathToDisplay := localPath
	if len(displayPath) > 0 && displayPath[0] != "" {
		pathToDisplay = displayPath[0]
	}

	fmt.Printf("Uploading %s to %s:%s\n", pathToDisplay, ip, remotePath)

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

func (s *SftpSender) Download(remotePath, ip, localLocation string) error {
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
func (s *SftpSender) uploadFileSFTP(client *ssh.Client, localPath, remotePath string) error {
	sftpClient, err := s.getSFTPClient(client)
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	// Create parent directories if they don't exist
	remoteDir := path.Dir(remotePath)
	if remoteDir != "." && remoteDir != "/" {
		if err := sftpClient.MkdirAll(remoteDir); err != nil {
			return fmt.Errorf("failed to create remote directory: %v", err)
		}
	}

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

	// Use io.CopyBuffer with optimal buffer size (256KB = 8x 32KB packet size)
	// This allows the SFTP library to optimize packet batching internally
	// Buffer size is a multiple of packet size for better alignment
	buffer := make([]byte, 256*1024) // 256KB = 8 packets, optimal for SFTP
	_, err = io.CopyBuffer(remoteFile, localFile, buffer)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

func (s *SftpSender) uploadDirectorySFTP(client *ssh.Client, localPath, remotePath string) error {
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

func (s *SftpSender) downloadSFTP(client *ssh.Client, remotePath, localPath string) error {
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

func (s *SftpSender) downloadFileSFTP(sftpClient *sftp.Client, remotePath, localPath string) error {
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

	// Use buffered writer for local file writes (helps with disk I/O)
	writer := bufio.NewWriterSize(localFile, 256*1024)
	defer writer.Flush()

	// Use io.CopyBuffer with optimal buffer size (256KB = 8x 32KB packet size)
	// This allows the SFTP library to optimize packet batching internally
	buffer := make([]byte, 256*1024) // 256KB = 8 packets, optimal for SFTP
	_, err = io.CopyBuffer(writer, remoteFile, buffer)
	if err != nil {
		return fmt.Errorf("failed to copy file content: %v", err)
	}

	return nil
}

func (s *SftpSender) downloadDirectorySFTP(sftpClient *sftp.Client, remotePath, localPath string) error {
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
func (s *SftpSender) getSSHClient(cred *Credential) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: cred.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(cred.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		// Optimize connection timeouts
		Timeout: 30 * time.Second,
	}

	// Create TCP connection with keepalive for better network handling
	// This helps maintain connection stability and reduces overhead
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", cred.IP), 30*time.Second)
	if err != nil {
		return nil, err
	}

	// Set TCP keepalive to maintain connection and detect dead connections faster
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
		// Set TCP no delay for lower latency (disable Nagle's algorithm)
		tcpConn.SetNoDelay(true)
	}

	// Perform SSH handshake with optimized connection
	c, chans, reqs, err := ssh.NewClientConn(conn, fmt.Sprintf("%s:22", cred.IP), config)
	if err != nil {
		conn.Close()
		return nil, err
	}

	return ssh.NewClient(c, chans, reqs), nil
}

func (s *SftpSender) getSFTPClient(sshClient *ssh.Client) (*sftp.Client, error) {
	// Create SFTP client with performance optimizations
	// Enable concurrent writes and reads for better performance (like Termius)
	// This allows multiple requests to be in flight simultaneously
	return sftp.NewClient(sshClient,
		sftp.UseConcurrentWrites(true),        // Enable concurrent writes - key for performance!
		sftp.UseConcurrentReads(true),         // Enable concurrent reads for downloads
		sftp.MaxConcurrentRequestsPerFile(64), // Allow up to 64 concurrent requests per file
	)
}

// parseWorkerNumbers parses autosend and ignore strings to return a sorted list of worker numbers
func parseWorkerNumbers(autosend, ignore string) ([]int, error) {
	if autosend == "" {
		return nil, fmt.Errorf("autosend cannot be empty")
	}

	// Parse ignore list first
	ignoreSet := make(map[int]bool)
	if ignore != "" {
		ignoreParts := strings.Split(ignore, ",")
		for _, part := range ignoreParts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid ignore number: %s", part)
			}
			ignoreSet[num] = true
		}
	}

	// Parse autosend (can be ranges or comma-separated)
	workerSet := make(map[int]bool)
	autosendParts := strings.Split(autosend, ",")
	for _, part := range autosendParts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a range (e.g., "21-27")
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid range format: %s (expected format: start-end)", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}

			if start > end {
				return nil, fmt.Errorf("range start (%d) must be <= end (%d)", start, end)
			}

			// Add all numbers in range
			for i := start; i <= end; i++ {
				if !ignoreSet[i] {
					workerSet[i] = true
				}
			}
		} else {
			// Single number
			num, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid worker number: %s", part)
			}
			if !ignoreSet[num] {
				workerSet[num] = true
			}
		}
	}

	// Convert set to sorted slice
	workers := make([]int, 0, len(workerSet))
	for num := range workerSet {
		workers = append(workers, num)
	}
	sort.Ints(workers)

	if len(workers) == 0 {
		return nil, fmt.Errorf("no workers to send to after applying ignore list")
	}

	return workers, nil
}

// findFileSequence extracts a number from the filename and finds the next files in sequence
func findFileSequence(basePath string, count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("count must be positive")
	}

	// Get absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Check if base file exists
	if _, err := os.Stat(absPath); err != nil {
		return nil, fmt.Errorf("base file does not exist: %s", absPath)
	}

	// Extract directory and filename
	dir := filepath.Dir(absPath)
	baseName := filepath.Base(absPath)

	// Extract number from filename using regex-like approach
	// Look for a number in the filename (e.g., "worker162.txt" -> 162)
	var baseNum int
	var prefix, suffix string
	found := false

	// Try to find a number in the filename
	for i := 0; i < len(baseName); i++ {
		if baseName[i] >= '0' && baseName[i] <= '9' {
			// Found start of number
			start := i
			end := i
			for end < len(baseName) && baseName[end] >= '0' && baseName[end] <= '9' {
				end++
			}
			numStr := baseName[start:end]
			num, err := strconv.Atoi(numStr)
			if err == nil {
				baseNum = num
				prefix = baseName[:start]
				suffix = baseName[end:]
				found = true
				break
			}
		}
	}

	if !found {
		return nil, fmt.Errorf("could not extract number from filename: %s", baseName)
	}

	// Generate file sequence
	files := make([]string, 0, count)
	for i := 0; i < count; i++ {
		num := baseNum + i
		fileName := fmt.Sprintf("%s%d%s", prefix, num, suffix)
		filePath := filepath.Join(dir, fileName)

		// Check if file exists
		if _, err := os.Stat(filePath); err != nil {
			return nil, fmt.Errorf("file does not exist in sequence: %s", filePath)
		}

		files = append(files, filePath)
	}

	return files, nil
}

// resolveWorkerName replaces * wildcard in IP template with worker{num}
func resolveWorkerName(workerNum int, ipTemplate string) string {
	// Replace * with worker{num}
	resolved := strings.ReplaceAll(ipTemplate, "*", fmt.Sprintf("worker%d", workerNum))
	return resolved
}

func main() {
	var (
		upload     = pflag.String("upload", "", "Local file/directory to upload")
		download   = pflag.String("download", "", "Remote file/directory to download")
		ip         = pflag.String("ip", "", "VPS IP address or name (required). Optionally include path: IP:/path or name:/path")
		configPath = pflag.String("config", "~/.config/sftpsender/config.yaml", "Path to config file")
		silent     = pflag.Bool("silent", false, "Silent mode.")
		version    = pflag.Bool("version", false, "Print the version of the tool and exit.")
		autosend   = pflag.String("autosend", "", "Automatically send files to workers. Accepts ranges (e.g., 21-27) or comma-separated numbers (e.g., 21,27)")
		ignore     = pflag.String("ignore", "", "Comma-separated worker numbers to exclude from autosend range")
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

	// Validate autosend usage
	if *autosend != "" && *download != "" {
		log.Fatal("--autosend can only be used with --upload, not with --download")
	}

	if *ip == "" {
		log.Fatal("IP address or VPS name is required. Use --ip flag")
	}

	if (*upload == "" && *download == "") || (*upload != "" && *download != "") {
		log.Fatal("You must specify either --upload or --download (but not both)")
	}

	// Ensure config file exists
	if err := ensureConfigExists(*configPath); err != nil {
		log.Fatalf("Failed to ensure config file exists: %v", err)
	}

	sftpsender, err := NewSftpSender(*configPath)
	if err != nil {
		log.Fatalf("Failed to initialize sftpsender: %v", err)
	}

	// Handle autosend mode
	if *autosend != "" && *upload != "" {
		// Parse worker numbers
		workers, err := parseWorkerNumbers(*autosend, *ignore)
		if err != nil {
			log.Fatalf("Failed to parse worker numbers: %v", err)
		}

		// Find file sequence
		files, err := findFileSequence(*upload, len(workers))
		if err != nil {
			log.Fatalf("Failed to find file sequence: %v", err)
		}

		// Validate file count matches worker count
		if len(files) != len(workers) {
			log.Fatalf("File count (%d) does not match worker count (%d)", len(files), len(workers))
		}

		// Get the original upload path's directory to preserve directory structure
		originalUploadDir := filepath.Dir(*upload)

		// Parse IP template and location
		ipParts := strings.SplitN(*ip, ":", 2)
		ipTemplate := ipParts[0]
		var location string
		if len(ipParts) > 1 {
			location = ipParts[1]
		}

		// Upload files to workers
		var errors []string
		successCount := 0
		for i, workerNum := range workers {
			// Resolve worker name from template
			workerName := resolveWorkerName(workerNum, ipTemplate)

			// Parse worker name and location
			workerParts := strings.SplitN(workerName, ":", 2)
			workerIPOrName := workerParts[0]
			workerLocation := location
			if len(workerParts) > 1 {
				workerLocation = workerParts[1]
			}

			// Construct display path preserving original directory structure
			// Use the original directory with the filename from the found file
			displayPath := filepath.Join(originalUploadDir, filepath.Base(files[i]))

			fmt.Printf("\n[%d/%d] Uploading to worker%d (%s)...\n", i+1, len(workers), workerNum, workerIPOrName)
			if err := sftpsender.Upload(files[i], workerIPOrName, workerLocation, displayPath); err != nil {
				errorMsg := fmt.Sprintf("Failed to upload to worker%d (%s): %v", workerNum, workerIPOrName, err)
				errors = append(errors, errorMsg)
				fmt.Printf("ERROR: %s\n", errorMsg)
			} else {
				successCount++
				fmt.Printf("✓ Successfully uploaded %s to worker%d\n", filepath.Base(files[i]), workerNum)
			}
		}

		// Print summary
		fmt.Printf("\n=== Upload Summary ===\n")
		fmt.Printf("Successful: %d/%d\n", successCount, len(workers))
		if len(errors) > 0 {
			fmt.Printf("Failed: %d/%d\n", len(errors), len(workers))
			fmt.Printf("\nErrors:\n")
			for _, errMsg := range errors {
				fmt.Printf("  - %s\n", errMsg)
			}
			log.Fatal("Some uploads failed")
		} else {
			fmt.Println("All uploads completed successfully!")
		}
	} else {
		// Original single-file upload/download logic
		// Parse IP/name and optional location from --ip flag
		// Format: IP or name:/path
		ipParts := strings.SplitN(*ip, ":", 2)
		ipOrName := ipParts[0]
		var location string
		if len(ipParts) > 1 {
			location = ipParts[1]
		}

		if *upload != "" {
			if err := sftpsender.Upload(*upload, ipOrName, location); err != nil {
				log.Fatalf("Upload failed: %v", err)
			}
			fmt.Println("Upload completed successfully!")
		} else if *download != "" {
			if err := sftpsender.Download(*download, ipOrName, location); err != nil {
				log.Fatalf("Download failed: %v", err)
			}
			fmt.Println("Download completed successfully!")
		}
	}
}
