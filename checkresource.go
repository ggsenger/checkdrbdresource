package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/LINBIT/drbdtop/pkg/resource"
	"github.com/LINBIT/drbdtop/pkg/update"
)

var resource_name string

func init() {
	flag.StringVar(&resource_name, "resource", "", "name of the DRBD resource to check")
	flag.Parse()
	if resource_name == "" {
		log.Fatal("please provide the name of the DRBD resource which should be checked e.g. '" + os.Args[0] + " --resource <resource name here>'")
	}

}

func getResourceState() *update.ByRes {
	res := update.NewByRes()
	out, err := exec.Command("drbdsetup", "events2", "--timestamps", "--statistics", "--now", resource_name).CombinedOutput()
	if err != nil {
		log.Fatalf("Error while retrieving resource status %v", err)
	}
	s := string(out)
	full_state := false
	for _, line := range strings.Split(s, "\n") {
		if strings.HasSuffix(line, "exists -") {
			res.Update(resource.NewEOF())
			full_state = true
			break
		}
		evt, err := resource.NewEvent(line)
		if err != nil {
			log.Fatalf("Error while parsing event %v", err)
		}
		res.Update(evt)
	}

	if !full_state {
		log.Fatalf("Unable to retrieve full state for resource " + resource_name)
	}
	return res
}

func main() {

	res := getResourceState()

	// do checks on resource
	res.RLock()
	defer res.RUnlock()

	if res.Res.Role != "Primary" {
		log.Fatal("Must be primary on resource " + resource_name + " for resize")
	}
	for _, c := range res.Connections {
		if c.ConnectionStatus != "Connected" || c.Role != "Secondary" {
			log.Fatal("Connection " + c.ConnectionName + "must be Connected/Secondary, but is " + c.ConnectionStatus + "/" + c.Role)
		}
	}
	for k, v := range res.Device.Volumes {
		if v.DiskState != "UpToDate" {
			log.Fatal("Local disk State for Volume " + resource_name + "/" + k + " is not UpToDate but" + v.DiskState)
		}
		for _, p := range res.PeerDevices {
			if vr, ok := p.Volumes[k]; ok {
				if vr.DiskState != "UpToDate" || vr.OutOfSyncKiB.Current != 0 {
					log.Fatal("Remote disk State for Volume " + resource_name + "/" + k + " is not UpToDate but" + vr.DiskState)
				}
			}
		}
	}

	os.Exit(0)
}
