package models

type Kubeconfig struct {
	APIVersion     string    `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Clusters       []Cluster `yaml:"clusters,omitempty" json:"clusters,omitempty"`
	Contexts       []Context `yaml:"contexts,omitempty" json:"contexts,omitempty"`
	CurrentContext string    `yaml:"current-context,omitempty" json:"current-context,omitempty"`
	Kind           string    `yaml:"kind,omitempty" json:"kind,omitempty"`
	Users          []User    `yaml:"users,omitempty" json:"users,omitempty"`
}

type Cluster struct {
	Name    string `yaml:"name,omitempty" json:"name,omitempty"`
	Cluster struct {
		Server string `yaml:"server,omitempty" json:"server,omitempty"`
	} `yaml:"cluster,omitempty" json:"cluster,omitempty"`
}

type Context struct {
	Name    string `yaml:"name,omitempty" json:"name,omitempty"`
	Context struct {
		Cluster   string `yaml:"cluster,omitempty" json:"cluster,omitempty"`
		Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
		User      string `yaml:"user,omitempty" json:"user,omitempty"`
	} `yaml:"context,omitempty" json:"context,omitempty"`
}

type User struct {
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	User struct {
		Token string `yaml:"token,omitempty" json:"token,omitempty"`
	} `yaml:"user,omitempty" json:"user,omitempty"`
}
