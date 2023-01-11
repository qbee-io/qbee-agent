package configuration

import (
	"bytes"
	"testing"

	"github.com/qbee-io/qbee-agent/app/configuration"
	"github.com/qbee-io/qbee-agent/test/runner"
)

func Test_PackageManagement_PreCondition(t *testing.T) {
	r := runner.New(t, runner.Debian)

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				PreCondition: "touch /pre-condition",
			},
		},
	}

	r.CreateJSON("config.json", config)

	r.MustExec("qbee-agent", "config", "-f", "config.json")

	if _, err := r.Exec("ls", "/pre-condition"); err != nil {
		t.Fatalf("expected file not found: %v", err)
	}
}

func Test_PackageManagement_InstallPackagePreConditionFailed(t *testing.T) {
	r := runner.New(t, runner.Debian)

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				PreCondition: "false",
				Packages: []configuration.Package{
					{
						Name: "austin",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output := r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check that no report is recorded
	expectedOutput := []byte("")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the newest version of test package is installed
	var err error
	output, err = r.Exec("austin", "--version")
	if err == nil {
		t.Fatalf("expected command to fail, got success")
	}

	if !bytes.Contains(output, []byte(`"austin": executable file not found`)) {
		t.Fatalf("expected `executable file not found` error, got %s", output)
	}
}

func Test_PackageManagement_InstallPackageNoPrecondition(t *testing.T) {
	r := runner.New(t, runner.Debian)

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				Packages: []configuration.Package{
					{
						Name: "austin",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output := r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput := []byte("[INFO] Package 'austin' successfully installed.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the newest version of test package is installed
	output = r.MustExec("austin", "--version")
	expectedOutput = []byte("austin 2.1.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_InstallPackagePreconditionSuccessful(t *testing.T) {
	r := runner.New(t, runner.Debian)

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				PreCondition: "true",
				Packages: []configuration.Package{
					{
						Name: "austin",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output := r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput := []byte("[INFO] Package 'austin' successfully installed.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the newest version of test package is installed
	output = r.MustExec("austin", "--version")
	expectedOutput = []byte("austin 2.1.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_InstallPackageWithVersion(t *testing.T) {
	r := runner.New(t, runner.Debian)

	r.MustExec("apt", "install", "austin")

	// check that the newest version of test package is installed
	output := r.MustExec("austin", "--version")
	expectedOutput := []byte("austin 2.1.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				Packages: []configuration.Package{
					{
						Name:    "austin",
						Version: "1.0.1-2",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output = r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput = []byte("[INFO] Package 'austin' successfully installed.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the correct version of test package is installed
	output = r.MustExec("austin", "--version")
	expectedOutput = []byte("austin 1.0.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_InstallPackageUpdateWithEmptyVersion(t *testing.T) {
	r := runner.New(t, runner.Debian)

	r.MustExec("apt", "install", "austin=1.0.1-2")

	// check that the newest version of test package is installed
	output := r.MustExec("austin", "--version")
	expectedOutput := []byte("austin 1.0.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				Packages: []configuration.Package{
					{
						Name: "austin",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output = r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput = []byte("[INFO] Package 'austin' successfully installed.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the correct version of test package is installed
	output = r.MustExec("austin", "--version")
	expectedOutput = []byte("austin 2.1.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_InstallPackageUpdateWithLatestVersion(t *testing.T) {
	r := runner.New(t, runner.Debian)

	r.MustExec("apt", "install", "austin=1.0.1-2")

	// check that the newest version of test package is installed
	output := r.MustExec("austin", "--version")
	expectedOutput := []byte("austin 1.0.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				Packages: []configuration.Package{
					{
						Name:    "austin",
						Version: "latest",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output = r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput = []byte("[INFO] Package 'austin' successfully installed.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the correct version of test package is installed
	output = r.MustExec("austin", "--version")
	expectedOutput = []byte("austin 2.1.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_InstallPackageUpdateWithReboot(t *testing.T) {
	r := runner.New(t, runner.Debian)

	r.MustExec("apt", "install", "austin=1.0.1-2")

	// check that the newest version of test package is installed
	output := r.MustExec("austin", "--version")
	expectedOutput := []byte("austin 1.0.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				RebootMode: configuration.RebootAlways,
				Packages: []configuration.Package{
					{
						Name: "austin",
					},
				},
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output = r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput = []byte("[INFO] Package 'austin' successfully installed.\n[WARN] Scheduling system reboot.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_UpgradeAll(t *testing.T) {
	r := runner.New(t, runner.Debian)

	r.MustExec("apt", "install", "austin=1.0.1-2")

	// check that the newest version of test package is installed
	output := r.MustExec("austin", "--version")
	expectedOutput := []byte("austin 1.0.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				FullUpgrade: true,
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output = r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput = []byte("[INFO] Full upgrade was successful - 1 packages updated.")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}

	// check that the correct version of test package is installed
	output = r.MustExec("austin", "--version")
	expectedOutput = []byte("austin 2.1.1")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}

func Test_PackageManagement_UpgradeAllWithReboot(t *testing.T) {
	r := runner.New(t, runner.Debian)

	r.MustExec("apt", "install", "austin=1.0.1-2")

	// check that the newest version of test package is installed
	output := r.MustExec("austin", "--version")
	expectedOut := []byte("austin 1.0.1")
	if !bytes.Equal(output, expectedOut) {
		t.Fatalf("expected %s, got %s", expectedOut, output)
	}

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				FullUpgrade: true,
				RebootMode:  configuration.RebootAlways,
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output = r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOut = []byte("[INFO] Full upgrade was successful - 1 packages updated.\n[WARN] Scheduling system reboot.")
	if !bytes.Equal(output, expectedOut) {
		t.Fatalf("expected %s, got %s", expectedOut, output)
	}
}

func Test_PackageManagement_UpgradeAllWithRebootWithoutChanges(t *testing.T) {
	r := runner.New(t, runner.Debian)

	config := configuration.CommittedConfig{
		Bundles: []string{"package_management"},
		BundleData: configuration.BundleData{
			PackageManagement: configuration.PackageManagementBundle{
				FullUpgrade: true,
				RebootMode:  configuration.RebootAlways,
			},
		},
	}

	// create config file in the container
	r.CreateJSON("config.json", config)

	// execute local configuration file
	output := r.MustExec("qbee-agent", "config", "-f", "config.json")

	// check if a correct report is recorded
	expectedOutput := []byte("")
	if !bytes.Equal(output, expectedOutput) {
		t.Fatalf("expected %s, got %s", expectedOutput, output)
	}
}
