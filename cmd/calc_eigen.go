package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"math/big"
)

var (
	rnethEigenRewardAmount string
)

func init() {
	calcEigenCmd.PersistentFlags().StringVarP(&rnethPointsInputPath, "rnethPointsInputPath", "", "", "rneth points input file path")
	calcEigenCmd.PersistentFlags().StringVarP(&rnethEigenRewardAmount, "rnethEigenRewardAmount", "", "", "ssv reward amount")
	calcEigenCmd.PersistentFlags().StringVarP(&outputDir, "outputDir", "", "", "output dir")
}

var calcEigenCmd = &cobra.Command{
	Use:     "calc-eigen",
	Short:   "calc eigen reward",
	Example: "./ssv-reward calc-eigen -h",
	Run: func(cmd *cobra.Command, args []string) {
		err := calcEigenReward()
		if err != nil {
			log.Error(err)
			return
		}
		log.Info("eigen reward calculation successful")
	},
}

func calcEigenReward() error {
	rnethPoints, err := getPoints(rnethPointsInputPath)
	if err != nil {
		return err
	}

	rnethEigenTotalAmount, isOk := big.NewInt(0).SetString(rnethEigenRewardAmount, 10)
	if !isOk {
		return fmt.Errorf("amount parsing failed")
	}

	rewardInfo, err := distribute(rnethPoints, rnethEigenTotalAmount)
	if err != nil {
		return err
	}

	if !check(rewardInfo, rnethEigenTotalAmount) {
		return fmt.Errorf("eigen reward check failed")
	}

	err = writeJson(rewardInfo, "final-eigen", outputDir)
	if err != nil {
		return err
	}

	return nil
}
