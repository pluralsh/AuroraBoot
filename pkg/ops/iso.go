package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kairos-io/AuroraBoot/pkg/schema"
	"github.com/kairos-io/kairos/pkg/utils"
	"github.com/otiai10/copy"
	"github.com/rs/zerolog/log"

	enkiaction "github.com/kairos-io/enki/pkg/action"
	"github.com/kairos-io/enki/pkg/types"
	enkitypes "github.com/kairos-io/enki/pkg/types"
	v1 "github.com/kairos-io/kairos-agent/v2/pkg/types/v1"
)

// GenISO generates an ISO from a rootfs, and stores results in dst
func GenISO(name, src, dst string, i schema.ISO) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		tmp, err := os.MkdirTemp("", "geniso")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		overlay := tmp
		if i.DataPath != "" {
			overlay = i.DataPath
		}

		err = copy.Copy(filepath.Join(dst, "config.yaml"), filepath.Join(overlay, "config.yaml"))
		if err != nil {
			return err
		}

		log.Info().Msgf("Generating iso '%s' from '%s' to '%s'", name, src, dst)
		cfg := &types.BuildConfig{
			Name:   name,
			OutDir: dst,
		}
		// Live grub artifacts:
		// https://github.com/kairos-io/osbuilder/blob/95509370f6a87229879f1a381afa5d47225ce12d/tools-image/Dockerfile#L29-L30
		spec := &enkitypes.LiveISO{
			RootFS:             []*v1.ImageSource{v1.NewDirSrc(src)},
			Image:              []*v1.ImageSource{v1.NewDirSrc("/efi"), v1.NewDirSrc("/grub2"), v1.NewDirSrc(overlay)},
			Label:              "COS_LIVE",
			GrubEntry:          "Kairos",
			BootloaderInRootFs: false,
		}
		buildISO := enkiaction.NewBuildISOAction(cfg, spec)
		err = buildISO.ISORun()
		if err != nil {
			cfg.Logger.Errorf(err.Error())
		}
		return err
	}
}

func InjectISO(dst, isoFile string, i schema.ISO) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		os.Chdir(dst)
		injectedIso := isoFile + ".custom.iso"
		os.Remove(injectedIso)

		tmp, err := os.MkdirTemp("", "injectiso")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)

		if i.DataPath != "" {
			log.Info().Msgf("Adding data in '%s' to '%s'", i.DataPath, isoFile)
			err = copy.Copy(i.DataPath, tmp)
			if err != nil {
				return err
			}
		}

		log.Info().Msgf("Adding cloud config file to '%s'", isoFile)
		err = copy.Copy(filepath.Join(dst, "config.yaml"), filepath.Join(tmp, "config.yaml"))
		if err != nil {
			return err
		}

		out, err := utils.SH(fmt.Sprintf("xorriso -indev %s -outdev %s -map %s / -boot_image any replay", isoFile, injectedIso, tmp))
		log.Print(out)
		if err != nil {
			return err
		}
		log.Info().Msgf("Wrote '%s'", injectedIso)
		return err
	}
}
