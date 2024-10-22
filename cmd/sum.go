package main

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"math/big"
)

var (
	total1PointsInputPath string
	total2PointsInputPath string
)

func init() {
	sumCmd.PersistentFlags().StringVarP(&total1PointsInputPath, "total1PointsInputPath", "", "", "total1 points input file path")
	sumCmd.PersistentFlags().StringVarP(&total2PointsInputPath, "total2PointsInputPath", "", "", "total2 points input file path")
	sumCmd.PersistentFlags().StringVarP(&outputDir, "outputDir", "", "", "output dir")
}

var sumCmd = &cobra.Command{
	Use:     "sum",
	Short:   "sum reward",
	Example: "./ssv-reward sum -h",
	Run: func(cmd *cobra.Command, args []string) {
		err := sumReward()
		if err != nil {
			log.Error(err)
			return
		}
		log.Info("sum successful")
	},
}

func sumReward() error {
	total1Points, err := getPoints(total1PointsInputPath)
	if err != nil {
		return err
	}

	total2Points, err := getPoints(total2PointsInputPath)
	if err != nil {
		return err
	}

	totalPoints := make(map[common.Address]*big.Int)
	for key, value := range total1Points {
		point, isOk := big.NewInt(0).SetString(value, 10)
		if !isOk {
			return fmt.Errorf("amount parsing failed")
		}
		addr := common.HexToAddress(key)
		totalPoints[addr] = point
	}
	for key2, value2 := range total2Points {
		addr2 := common.HexToAddress(key2)
		point2, isOk := big.NewInt(0).SetString(value2, 10)
		if !isOk {
			return fmt.Errorf("amount parsing failed")
		}
		if _, ok := totalPoints[addr2]; ok {
			totalPoints[addr2] = big.NewInt(0).Add(totalPoints[addr2], point2)
		} else {
			totalPoints[addr2] = point2
		}
	}

	err = writeJson(totalPoints, "total-final", outputDir)
	if err != nil {
		return err
	}

	return nil
}
