package main

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/docker/docker/client"

	"github.com/BurntSushi/toml"

	"github.com/buildpacks/lifecycle"
	"github.com/buildpacks/lifecycle/cmd"
	"github.com/buildpacks/lifecycle/env"
	"github.com/buildpacks/lifecycle/priv"
)

type rootBuildCmd struct {
	// flags: inputs
	groupPath string
	planPath  string
	rootBuildArgs
}

type rootBuildArgs struct {
	// inputs needed when run by creator
	buildpacksDir string
	platformDir   string
	useDaemon     bool
	uid, gid      int

	//construct if necessary before dropping privileges
	docker client.CommonAPIClient
}

func (b *rootBuildCmd) Init() {
	cmd.FlagBuildpacksDir(&b.buildpacksDir)
	cmd.FlagGroupPath(&b.groupPath)
	cmd.FlagPlanPath(&b.planPath)
	cmd.FlagPlatformDir(&b.platformDir)
}

func (b *rootBuildCmd) Args(nargs int, args []string) error {
	if nargs != 0 {
		return cmd.FailErrCode(errors.New("received unexpected arguments"), cmd.CodeInvalidArgs, "parse arguments")
	}
	return nil
}

func (b *rootBuildCmd) Privileges() error {
	if b.useDaemon {
		var err error
		b.docker, err = priv.DockerClient()
		if err != nil {
			return cmd.FailErr(err, "initialize docker client")
		}
	}
	if err := priv.RunAs(b.uid, b.gid); err != nil {
		cmd.FailErr(err, fmt.Sprintf("exec as user %d:%d", b.uid, b.gid))
	}
	if err := priv.SetEnvironmentForUser(b.uid); err != nil {
		cmd.FailErr(err, fmt.Sprintf("set environment for user %d", b.uid))
	}
	return nil
}

func (b *rootBuildCmd) Exec() error {
	group, plan, err := b.readData()
	if err != nil {
		return err
	}
	return b.build(group, plan)
}

func (ba rootBuildArgs) build(group lifecycle.BuildpackGroup, plan lifecycle.BuildPlan) error {
	builder := &lifecycle.RootBuilder{
		RootDir:       "/",
		PlatformDir:   ba.platformDir,
		BuildpacksDir: ba.buildpacksDir,
		Env:           env.NewBuildEnv(os.Environ()),
		Group:         group,
		Plan:          plan,
		Out:           log.New(os.Stdout, "", 0),
		Err:           log.New(os.Stderr, "", 0),
	}
	_, err := builder.Build()
	if err != nil {
		return cmd.FailErrCode(err, cmd.CodeFailedBuild, "build")
	}

	return nil
}

func (b *rootBuildCmd) readData() (lifecycle.BuildpackGroup, lifecycle.BuildPlan, error) {
	group, err := lifecycle.ReadGroup(b.groupPath)
	if err != nil {
		return lifecycle.BuildpackGroup{}, lifecycle.BuildPlan{}, cmd.FailErr(err, "read buildpack group")
	}

	var plan lifecycle.BuildPlan
	if _, err := toml.DecodeFile(b.planPath, &plan); err != nil {
		return lifecycle.BuildpackGroup{}, lifecycle.BuildPlan{}, cmd.FailErr(err, "parse detect plan")
	}
	return group, plan, nil
}