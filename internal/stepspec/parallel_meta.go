package stepspec

import "github.com/Airgap-Castaways/deck/internal/stepmeta"

func parallelApplySafe(paths ...string) stepmeta.ParallelMetadata {
	return stepmeta.ParallelMetadata{ApplySafe: true, ApplyTargetPaths: paths}
}

func parallelTargetPaths(paths ...string) stepmeta.ParallelMetadata {
	return stepmeta.ParallelMetadata{ApplyTargetPaths: paths}
}

func parallelPrepareOutput(path, root, example string) stepmeta.ParallelMetadata {
	return stepmeta.ParallelMetadata{PrepareOutput: stepmeta.OutputRootConstraint{Path: path, Root: root, Example: example}}
}

func withParallel(def stepmeta.Definition, parallel stepmeta.ParallelMetadata) stepmeta.Definition {
	def.Parallel = parallel
	return def
}
