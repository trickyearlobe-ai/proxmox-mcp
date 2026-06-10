package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/diskfs/go-diskfs/backend/file"
	"github.com/diskfs/go-diskfs/filesystem/iso9660"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// buildAnswerISO assembles an ISO9660 image with the given volume label (the label
// installers use to auto-discover the answer file) and file contents.
//
// Rock Ridge and Joliet extensions are both enabled so the exact, case-sensitive
// filenames survive: cloud-init's NoCloud datasource needs literal "user-data" /
// "meta-data" (Rock Ridge, for Linux), and Windows Setup needs literal
// "autounattend.xml" (Joliet). Plain ISO9660 would mangle these to 8.3 uppercase.
func buildAnswerISO(label string, files map[string][]byte) ([]byte, error) {
	workspace, err := os.MkdirTemp("", "answer-iso-ws-")
	if err != nil {
		return nil, fmt.Errorf("creating ISO workspace: %w", err)
	}
	defer os.RemoveAll(workspace)

	out, err := os.CreateTemp("", "answer-*.iso")
	if err != nil {
		return nil, fmt.Errorf("creating temp ISO: %w", err)
	}
	defer os.Remove(out.Name())
	defer out.Close()

	bk := file.New(out, false)
	fs, err := iso9660.Create(bk, 0, 0, 2048, workspace)
	if err != nil {
		return nil, fmt.Errorf("creating ISO filesystem: %w", err)
	}

	for name, data := range files {
		f, err := fs.OpenFile("/"+name, os.O_CREATE|os.O_RDWR)
		if err != nil {
			return nil, fmt.Errorf("creating %s in ISO: %w", name, err)
		}
		if _, err := f.Write(data); err != nil {
			return nil, fmt.Errorf("writing %s to ISO: %w", name, err)
		}
	}

	if err := fs.Finalize(iso9660.FinalizeOptions{
		RockRidge:        true,
		Joliet:           true,
		VolumeIdentifier: label,
	}); err != nil {
		return nil, fmt.Errorf("finalizing ISO: %w", err)
	}

	if _, err := out.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewinding ISO: %w", err)
	}
	return io.ReadAll(out)
}

// answerLayout maps an install method to the volume label and primary filename the
// matching installer auto-discovers.
//
//   - kickstart   → anaconda loads ks.cfg from a filesystem labeled OEMDRV
//   - nocloud     → cloud-init / Ubuntu autoinstall read a CIDATA NoCloud datasource
//   - autounattend→ Windows Setup scans removable media for autounattend.xml
func answerLayout(kind string) (label, primaryFile string, ok bool) {
	switch strings.ToLower(kind) {
	case "kickstart":
		return "OEMDRV", "ks.cfg", true
	case "nocloud":
		return "CIDATA", "user-data", true
	case "autounattend":
		return "ANSWER", "autounattend.xml", true
	default:
		return "", "", false
	}
}

type CreateAnswerMediaInput struct {
	Host          string `json:"host,omitempty" jsonschema:"Proxmox host name from config (uses default if omitted)"`
	Node          string `json:"node" jsonschema:"node to upload the generated ISO to,required"`
	Storage       string `json:"storage" jsonschema:"target storage for the ISO (must allow 'iso' content, e.g. local),required"`
	Filename      string `json:"filename" jsonschema:"destination ISO filename, e.g. answer-9000.iso,required"`
	Kind          string `json:"kind" jsonschema:"unattended method: 'kickstart' (RHEL/Fedora), 'nocloud' (Ubuntu autoinstall or cloud-init), or 'autounattend' (Windows),required"`
	Content       string `json:"content" jsonschema:"the answer file body the LLM authored: ks.cfg for kickstart, cloud-init user-data for nocloud, or autounattend.xml for autounattend,required"`
	MetaData      string `json:"meta_data,omitempty" jsonschema:"nocloud only: meta-data file content (defaults to a minimal instance-id if omitted)"`
	NetworkConfig string `json:"network_config,omitempty" jsonschema:"nocloud only: optional network-config file content"`
}

type CreateAnswerMediaOutput struct {
	Host    string `json:"host" jsonschema:"Proxmox host that was queried"`
	Node    string `json:"node" jsonschema:"node the ISO was uploaded to"`
	Storage string `json:"storage" jsonschema:"target storage"`
	VolID   string `json:"volid" jsonschema:"volume ID to attach as a CD via update_vm_config (e.g. local:iso/answer-9000.iso,media=cdrom)"`
	Label   string `json:"label" jsonschema:"volume label the installer uses to auto-discover the answer file"`
	UPID    string `json:"upid" jsonschema:"upload task UPID — track with get_task_status"`
}

func createAnswerMediaHandler(reg *HostRegistry) func(context.Context, *mcp.CallToolRequest, CreateAnswerMediaInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input CreateAnswerMediaInput) (*mcp.CallToolResult, any, error) {
		client, host, err := reg.GetClient(input.Host)
		if err != nil {
			return nil, CreateAnswerMediaOutput{}, err
		}
		if err := confirmWrite(ctx, reg, req, fmt.Sprintf("create_answer_media: upload %s answer ISO %s to %s/%s", input.Kind, input.Filename, host, input.Storage)); err != nil {
			return nil, CreateAnswerMediaOutput{}, err
		}

		label, primary, ok := answerLayout(input.Kind)
		if !ok {
			return nil, CreateAnswerMediaOutput{}, fmt.Errorf("unknown kind %q (expected kickstart, nocloud, or autounattend)", input.Kind)
		}

		files := map[string][]byte{primary: []byte(input.Content)}
		if strings.EqualFold(input.Kind, "nocloud") {
			meta := input.MetaData
			if meta == "" {
				meta = "instance-id: iid-local01\n"
			}
			files["meta-data"] = []byte(meta)
			if input.NetworkConfig != "" {
				files["network-config"] = []byte(input.NetworkConfig)
			}
		}

		data, err := buildAnswerISO(label, files)
		if err != nil {
			return nil, CreateAnswerMediaOutput{}, err
		}

		upid, err := client.Upload(ctx, input.Node, input.Storage, "iso", input.Filename, data)
		if err != nil {
			return nil, CreateAnswerMediaOutput{}, fmt.Errorf("uploading answer ISO: %w", err)
		}

		return nil, CreateAnswerMediaOutput{
			Host:    host,
			Node:    input.Node,
			Storage: input.Storage,
			VolID:   fmt.Sprintf("%s:iso/%s", input.Storage, input.Filename),
			Label:   label,
			UPID:    upid,
		}, nil
	}
}

func RegisterAnswerMediaTools(server *mcp.Server, reg *HostRegistry) {
	mcp.AddTool[CreateAnswerMediaInput, any](server, &mcp.Tool{
		Name: "create_answer_media",
		Description: "Package an unattended-install answer file (kickstart/cloud-init/autounattend) into a small labeled ISO and upload it to storage. " +
			"Attach the returned volid as a second CD via update_vm_config so the installer auto-discovers it. " +
			"The installer still needs its main ISO attached too; for Windows you also need a virtio-win CD.",
	}, createAnswerMediaHandler(reg))
}
