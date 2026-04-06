package install

import (
	"context"

	"github.com/Airgap-Castaways/deck/internal/stepspec"
)

type kubeadmExecutor interface {
	Init(context.Context, stepspec.KubeadmInit) error
	Join(context.Context, stepspec.KubeadmJoin) error
	Reset(context.Context, stepspec.KubeadmReset) error
	Upgrade(context.Context, stepspec.KubeadmUpgrade) error
}

type realKubeadmExecutor struct{}

func (realKubeadmExecutor) Init(ctx context.Context, spec stepspec.KubeadmInit) error {
	return runInitKubeadmReal(ctx, spec)
}

func (realKubeadmExecutor) Join(ctx context.Context, spec stepspec.KubeadmJoin) error {
	return runJoinKubeadmReal(ctx, spec)
}

func (realKubeadmExecutor) Reset(ctx context.Context, spec stepspec.KubeadmReset) error {
	return runResetKubeadmReal(ctx, spec)
}

func (realKubeadmExecutor) Upgrade(ctx context.Context, spec stepspec.KubeadmUpgrade) error {
	return runUpgradeKubeadmReal(ctx, spec)
}

func withKubeadmExecutor(executor kubeadmExecutor) kubeadmExecutor {
	if executor != nil {
		return executor
	}
	return realKubeadmExecutor{}
}
