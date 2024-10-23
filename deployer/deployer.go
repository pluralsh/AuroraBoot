package deployer

import (
	"context"
	"os"

	"github.com/hashicorp/go-multierror"
	"github.com/kairos-io/AuroraBoot/pkg/schema"

	"github.com/rs/zerolog/log"
	"github.com/spectrocloud-labs/herd"
	"gopkg.in/yaml.v3"
)

type Deployer struct {
	*herd.Graph
	config   schema.Config
	artifact schema.ReleaseArtifact
}

func NewDeployer(c schema.Config, a schema.ReleaseArtifact) *Deployer {
	d := &Deployer{config: c, artifact: a}
	d.Graph = herd.DAG(herd.EnableInit)

	return d
}

func (d *Deployer) CollectErrors() error {
	var err error
	for _, layer := range d.Analyze() {
		for _, op := range layer {
			if op.Error != nil {
				err = multierror.Append(err, op.Error)
			}
		}
	}

	return err
}

func LoadByte(b []byte) (*schema.Config, *schema.ReleaseArtifact, error) {
	config := &schema.Config{}
	release := &schema.ReleaseArtifact{}

	if err := yaml.Unmarshal(b, config); err != nil {
		return nil, nil, err
	}

	if err := yaml.Unmarshal(b, release); err != nil {
		return nil, nil, err
	}

	return config, release, nil
}

// LoadFile loads a configuration file and returns the AuroraBoot configuration
// and release artifact information
func LoadFile(file string) (*schema.Config, *schema.ReleaseArtifact, error) {

	dat, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}

	return LoadByte(dat)
}

// Start starts the auroraboot deployer
func Start(config *schema.Config, release *schema.ReleaseArtifact) error {

	f, err := os.CreateTemp("", "auroraboot-dat")
	if err != nil {
		return err
	}

	_, err = f.WriteString(config.CloudConfig)
	if err != nil {
		return err
	}

	// Have a dag for our ops
	g := herd.DAG(herd.CollectOrphans)

	Register(g, *release, *config, f.Name())

	writeDag(g.Analyze())

	ctx := context.Background()
	err = g.Run(ctx)
	if err != nil {
		return err
	}

	for _, layer := range g.Analyze() {
		for _, op := range layer {
			if op.Error != nil {
				err = multierror.Append(err, op.Error)
			}
		}
	}

	return err
}

func writeDag(d [][]herd.GraphEntry) {
	for i, layer := range d {
		log.Printf("%d.", (i + 1))
		for _, op := range layer {
			if !op.Ignored {
				if op.Error != nil {
					log.Printf(" <%s> (error: %s) (background: %t)", op.Name, op.Error.Error(), op.Background)
				} else {
					log.Printf(" <%s> (background: %t)", op.Name, op.Background)
				}
			}
		}
		log.Print("")
	}
}
