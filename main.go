package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/trickyearlobe-ai/proxmox-mcp/config"
	"github.com/trickyearlobe-ai/proxmox-mcp/install"
	"github.com/trickyearlobe-ai/proxmox-mcp/tools"
)

var version = "dev"

func main() {
	log.SetOutput(os.Stderr) // Never log to stdout on stdio transport

	initFlag := flag.Bool("init", false, "Create a template config file at ~/.proxmox.yaml and exit")
	installFlag := flag.Bool("install", false, "Register as MCP server in all detected IDEs and exit")
	uninstallFlag := flag.Bool("uninstall", false, "Remove from all IDE MCP configs and exit")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Fprintf(os.Stderr, "proxmox-mcp %s\n", version)
		os.Exit(0)
	}

	if *initFlag {
		if err := config.Init(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *installFlag {
		fmt.Fprintf(os.Stderr, "Installing proxmox-mcp in detected IDEs...\n")
		if err := install.Install(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	if *uninstallFlag {
		fmt.Fprintf(os.Stderr, "Removing proxmox-mcp from IDE configs...\n")
		if err := install.Uninstall(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// Normal MCP server mode
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	reg := tools.NewHostRegistry(cfg)
	if strings.EqualFold(os.Getenv("PROXMOX_CONFIRM_WRITES"), "true") {
		reg.SetConfirmWrites(true)
		log.Printf("write confirmation enabled: mutating tools will prompt for approval via elicitation")
	}

	hostNames := reg.HostNames()
	hostList := ""
	if len(hostNames) > 0 {
		hostList = " Available hosts: " + fmt.Sprintf("%v", hostNames) + "."
	}

	server := mcp.NewServer(
		&mcp.Implementation{
			Name:    "proxmox-mcp",
			Version: version,
		},
		&mcp.ServerOptions{
			Instructions: "Proxmox VE MCP server. Manage virtual machines, containers, nodes, storage, and cluster resources. " +
				"For VM/container actions (start/stop/shutdown), the returned UPID can be checked with get_task_status. " +
				"Node is auto-resolved from VMID when omitted. Use the 'host' parameter to target a specific Proxmox host." +
				hostList,
		},
	)

	tools.RegisterClusterTools(server, reg)
	tools.RegisterNodeTools(server, reg)
	tools.RegisterVMTools(server, reg)
	tools.RegisterContainerTools(server, reg)
	tools.RegisterStorageTools(server, reg)
	tools.RegisterProvisioningTools(server, reg)
	tools.RegisterAnswerMediaTools(server, reg)
	tools.RegisterWaitTools(server, reg)
	tools.RegisterConsoleTools(server, reg)
	tools.RegisterTeardownTools(server, reg)
	tools.RegisterRawTools(server, reg)

	log.Printf("proxmox-mcp %s starting (%d host(s) configured)", version, len(hostNames))

	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}
}
