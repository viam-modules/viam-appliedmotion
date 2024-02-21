package main

import (
	"context"

	"github.com/edaniels/golog"
	"go.viam.com/rdk/components/motor"
	"go.viam.com/rdk/module"
	"go.viam.com/utils"

	"viam-labs/viam-appliedmotion/st"
)

func main() {
	utils.ContextualMain(mainWithArgs, module.NewLoggerFromArgs("appliedmotion"))
}

func mainWithArgs(ctx context.Context, args []string, logger golog.Logger) (err error) {
	custom_module, err := module.NewModuleFromArgs(ctx, logger)
	if err != nil {
		return err
	}

	err = custom_module.AddModelFromRegistry(ctx, motor.API, st.Model)
	if err != nil {
		return err
	}

	err = custom_module.Start(ctx)
	defer custom_module.Close(ctx)
	if err != nil {
		return err
	}

	<-ctx.Done()
	return nil
}
