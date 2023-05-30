package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dustin/go-humanize"
	"github.com/manifoldco/promptui"
	"github.com/oursky/pageship/internal/api"
	"github.com/oursky/pageship/internal/config"
	"github.com/oursky/pageship/internal/deploy"
	"github.com/oursky/pageship/internal/models"
	"github.com/oursky/pageship/internal/time"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(deployCmd)

	deployCmd.PersistentFlags().String("site", "", "site to deploy")
	deployCmd.PersistentFlags().String("name", "", "deployment name; autogenerated if not set")
	deployCmd.PersistentFlags().BoolP("yes", "y", false, "skip confirmation")
}

func packTar(dir string, tarfile *os.File, conf *config.Config) ([]models.FileEntry, int64, error) {
	modTime := time.SystemClock.Now()
	collector, err := deploy.NewCollector(modTime, tarfile)
	if err != nil {
		return nil, 0, err
	}
	defer collector.Close()

	collector.AddDir("/")

	publicDir := filepath.Join(dir, conf.Site.Public)
	conf.Site.Public = "public"

	confJSON, err := json.MarshalIndent(conf, "", "\t")
	if err != nil {
		return nil, 0, err
	}
	err = collector.AddFile(fmt.Sprintf("/%s.json", config.SiteConfigName), confJSON)
	if err != nil {
		return nil, 0, err
	}

	err = collector.Collect(os.DirFS(publicDir), "/public")
	if err != nil {
		return nil, 0, fmt.Errorf("collecting from %s: %w", publicDir, err)
	}

	collector.Close()

	_, err = tarfile.Seek(0, io.SeekStart)
	if err != nil {
		return nil, 0, err
	}

	fi, err := tarfile.Stat()
	if err != nil {
		return nil, 0, err
	}

	return collector.Files(), fi.Size(), nil
}

func doDeploy(ctx context.Context, appID string, siteName string, deploymentName string, conf *config.Config, dir string) {
	tarfile, err := os.CreateTemp("", fmt.Sprintf("pageship-%s-%s-*.tar.zst", appID, deploymentName))
	if err != nil {
		Error("Failed to create temp file: %s", err)
		return
	}
	defer os.Remove(tarfile.Name())

	Info("Collecting files...")
	Debug("Tarball: %s", tarfile.Name())
	files, tarSize, err := packTar(dir, tarfile, conf)
	if err != nil {
		Error("Failed to collect files: %s", err)
		return
	}

	Info("%d files found. Tarball size: %s", len(files), humanize.Bytes(uint64(tarSize)))

	Debug("Configuring app...")
	_, err = apiClient.ConfigureApp(ctx, appID, &conf.App)
	if err != nil {
		Error("Failed to configure app: %s", err)
		return
	}

	Info("Setting up deployment '%s'...", deploymentName)

	if siteName != "" {
		site, err := apiClient.CreateSite(ctx, appID, siteName)
		if err != nil {
			Error("Failed to setup site: %s", err)
			return
		}
		Debug("Site ID: %s", site.ID)
		lastDeploymentName := "-"
		if site.DeploymentName != nil {
			lastDeploymentName = *site.DeploymentName
		}
		Debug("Last Deployment Name: %s", lastDeploymentName)
	} else {
		Info("Site not specified; deployment would not be assigned to site")
	}

	deployment, err := apiClient.SetupDeployment(ctx, appID, deploymentName, files, &conf.Site)
	if err != nil {
		Error("Failed to setup deployment: %s", err)
		return
	}

	Debug("Deployment ID: %s", deployment.ID)

	bar := progressbar.DefaultBytes(tarSize, "uploading")
	body := io.TeeReader(tarfile, bar)
	deployment, err = apiClient.UploadDeploymentTarball(ctx, appID, deployment.Name, body)
	if err != nil {
		Error("Failed to upload tarball: %s", err)
		return
	}

	if siteName != "" {
		Info("Activating deployment...")
		_, err = apiClient.UpdateSite(ctx, appID, siteName, &api.SitePatchRequest{
			DeploymentName: &deployment.Name,
		})
		if err != nil {
			Error("Failed to activate deployment: %s", err)
			return
		}
	}

	d, err := apiClient.GetDeployment(ctx, appID, deploymentName)
	if err != nil {
		Error("Failed to get deployment: %s", err)
		return
	}

	if d.URL != nil {
		Info("You can access the deployment at: %s", *d.URL)
	}

	Info("Done!")
}

var deployCmd = &cobra.Command{
	Use:   "deploy [deploy directory] [--site site to deploy] [--name deployment name] [--yes]",
	Short: "Deploy site",
	Run: func(cmd *cobra.Command, args []string) {
		site := viper.GetString("site")
		name := viper.GetString("name")
		yes := viper.GetBool("yes")

		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}

		dir, err := filepath.Abs(dir)
		if err != nil {
			Error("Invalid deploy directory: %s", err)
			return
		}

		if name == "" {
			name = models.RandomID(4)
		}
		if !config.ValidateDNSLabel(name) {
			Error("Invalid deployment name: must be a valid DNS label")
			return
		}

		loader := config.NewLoader(config.SiteConfigName)

		conf := config.DefaultConfig()
		if err := loader.Load(os.DirFS(dir), &conf); err != nil {
			Error("Failed to load config: %s", err)
			return
		}
		conf.SetDefaults()

		appID := conf.App.ID
		if site != "" {
			_, ok := conf.App.ResolveSite(site)
			if !ok {
				Error("Site is not defined: %s", site)
				return
			}
		}

		if !yes {
			var label string
			if site == "" {
				label = fmt.Sprintf("Deploy to app %q?", appID)
			} else {
				label = fmt.Sprintf("Deploy to site %q of app %q?", site, appID)
			}

			prompt := promptui.Prompt{Label: label, IsConfirm: true}
			_, err := prompt.Run()
			if err != nil {
				Info("Cancelled.")
				return
			}
		}

		doDeploy(cmd.Context(), appID, site, name, &conf, dir)
	},
}
