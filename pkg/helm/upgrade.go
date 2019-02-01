package helm

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

// UpgradeStep represents the structure of an Upgrade step
type UpgradeStep struct {
	Description string           `yaml:"description"`
	Outputs     []HelmOutput     `yaml:"outputs"`
	Arguments   UpgradeArguments `yaml:"helm"`
}

// UpgradeArguments represent the arguments available to the Upgrade step
type UpgradeArguments struct {
	Namespace   string            `yaml:"namespace"`
	Name        string            `yaml:"name"`
	Chart       string            `yaml:"chart"`
	Version     string            `yaml:"version"`
	Set         map[string]string `yaml:"set"`
	Values      []string          `yaml:"values"`
	Wait        bool              `yaml:"wait"`
	ResetValues bool              `yaml:"resetValues"`
	ReuseValues bool              `yaml:"reuseValues"`
}

// Upgrade issues a helm upgrade command for a release using the provided UpgradeArguments
func (m *Mixin) Upgrade() error {
	payload, err := m.getPayloadData()
	if err != nil {
		return err
	}

	kubeClient, err := m.getKubernetesClient("/root/.kube/config")
	if err != nil {
		return errors.Wrap(err, "couldn't get kubernetes client")
	}

	var step UpgradeStep
	err = yaml.Unmarshal(payload, &step)
	if err != nil {
		return err
	}

	cmd := m.NewCommand("helm", "upgrade", step.Arguments.Name, step.Arguments.Chart)

	if step.Arguments.Namespace != "" {
		cmd.Args = append(cmd.Args, "--namespace", step.Arguments.Namespace)
	}

	if step.Arguments.Version != "" {
		cmd.Args = append(cmd.Args, "--version", step.Arguments.Version)
	}

	if step.Arguments.ResetValues {
		cmd.Args = append(cmd.Args, "--reset-values")
	}

	if step.Arguments.ReuseValues {
		cmd.Args = append(cmd.Args, "--reuse-values")
	}

	if step.Arguments.Wait {
		cmd.Args = append(cmd.Args, "--wait")
	}

	for _, v := range step.Arguments.Values {
		cmd.Args = append(cmd.Args, "--values", v)
	}

	// sort the set consistently
	setKeys := make([]string, 0, len(step.Arguments.Set))
	for k := range step.Arguments.Set {
		setKeys = append(setKeys, k)
	}
	sort.Strings(setKeys)

	for _, k := range setKeys {
		cmd.Args = append(cmd.Args, "--set", fmt.Sprintf("%s=%s", k, step.Arguments.Set[k]))
	}

	cmd.Stdout = m.Out
	cmd.Stderr = m.Err

	prettyCmd := fmt.Sprintf("%s %s", cmd.Path, strings.Join(cmd.Args, " "))
	fmt.Fprintln(m.Out, prettyCmd)

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("could not execute command, %s: %s", prettyCmd, err)
	}
	err = cmd.Wait()
	if err != nil {
		return err
	}

	var lines []string
	for _, output := range step.Outputs {
		val, err := getSecret(kubeClient, step.Arguments.Namespace, output.Secret, output.Key)
		if err != nil {
			return err
		}
		l := fmt.Sprintf("%s=%s", output.Name, val)
		lines = append(lines, l)

	}
	m.Context.WriteOutput(lines)
	return nil
}
