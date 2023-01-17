package main

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v3"
	"io"
	"log"
	"net"
	"os"
	"strings"
)

type Config struct {
	Clients []Client
}

type Client struct {
	SSH     SSHConfig
	Forward []struct {
		LocalPort  string `yaml:"local"`
		RemotePort string `yaml:"remote"`
		Forwarder  *Forwarder
	} `yaml:"forwards"`
}

type SSHConfig struct {
	Host     string
	Port     string
	User     string
	Password string
}

type Forwarder struct {
	localPort  string
	remotePort string

	sshClient *ssh.Client
}

func (f *Forwarder) run() {
	log.Println("Tunnel running with", "localhost:"+f.localPort)
	listener, err := net.Listen("tcp", "localhost:"+f.localPort)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		local, err := listener.Accept()
		if err != nil {
			log.Println("accept local error:", err)
			continue
		}
		remote, err := f.sshClient.Dial("tcp", "localhost:"+f.remotePort)
		if err != nil {
			log.Println("remote dial error:", err)
			continue
		}
		runTunnel(local, remote)
	}
}

var config = &Config{}

func init() {
	file, err := os.ReadFile("config.yml")
	if err != nil {
		log.Fatal("reading config error")
	}
	err = yaml.NewDecoder(strings.NewReader(string(file))).Decode(config)
}

func main() {
	for _, c := range config.Clients {
		client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%s", c.SSH.Host, c.SSH.Port), &ssh.ClientConfig{
			User:            c.SSH.User,
			Auth:            []ssh.AuthMethod{ssh.Password(c.SSH.Password)},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		})
		if err != nil {
			log.Fatalf("SSH dial error: %s", err.Error())
		}
		for _, fp := range c.Forward {
			fw := &Forwarder{
				localPort:  fp.LocalPort,
				remotePort: fp.RemotePort,
				sshClient:  client,
			}
			fp.Forwarder = fw
			go fw.run()
		}
	}

	select {}
}

func runTunnel(local, remote net.Conn) {
	defer local.Close()
	defer remote.Close()
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()

	<-done
}
