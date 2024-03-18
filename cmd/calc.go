package main

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

var (
	nethPointsInputPath  string
	rnethPointsInputPath string
	nethSsvRewardAmount  string
	rnethSsvRewardAmount string
	outputDir            string
)

func init() {
	calcCmd.PersistentFlags().StringVarP(&nethPointsInputPath, "nethPointsInputPath", "", "", "neth points input file path")
	calcCmd.PersistentFlags().StringVarP(&rnethPointsInputPath, "rnethPointsInputPath", "", "", "rneth points input file path")
	calcCmd.PersistentFlags().StringVarP(&nethSsvRewardAmount, "nethSsvRewardAmount", "", "", "ssv reward amount")
	calcCmd.PersistentFlags().StringVarP(&rnethSsvRewardAmount, "rnethSsvRewardAmount", "", "", "ssv reward amount")
	calcCmd.PersistentFlags().StringVarP(&outputDir, "outputDir", "", "", "output dir")
}

var calcCmd = &cobra.Command{
	Use:     "calc",
	Short:   "calc reward",
	Example: "./ssv-reward calc -h",
	Run: func(cmd *cobra.Command, args []string) {
		err := calcReward()
		if err != nil {
			log.Error(err)
			return
		}
		log.Info("reward calculation successful")
	},
}

func calcReward() error {
	nethPoints, err := getPoints(nethPointsInputPath)
	if err != nil {
		return err
	}

	rnethPoints, err := getPoints(rnethPointsInputPath)
	if err != nil {
		return err
	}

	nethTotalAmount, isOk := big.NewInt(0).SetString(nethSsvRewardAmount, 10)
	if !isOk {
		return fmt.Errorf("amount parsing failed")
	}

	nethRewardInfo, err := distribute(nethPoints, nethTotalAmount)
	if err != nil {
		return err
	}

	if !check(nethRewardInfo, nethTotalAmount) {
		return fmt.Errorf("neth reward check failed")
	}

	rnethTotalAmount, isOk := big.NewInt(0).SetString(rnethSsvRewardAmount, 10)
	if !isOk {
		return fmt.Errorf("amount parsing failed")
	}

	rnethRewardInfo, err := distribute(rnethPoints, rnethTotalAmount)
	if err != nil {
		return err
	}

	if !check(rnethRewardInfo, rnethTotalAmount) {
		return fmt.Errorf("rneth reward check failed")
	}

	finalRewardInfo := map[common.Address]*big.Int{}
	for key, value := range nethRewardInfo {
		key2 := key
		value2 := *value
		finalRewardInfo[key2] = &value2
	}

	for key, value := range rnethRewardInfo {
		v, ok := finalRewardInfo[key]
		if ok {
			finalRewardInfo[key] = big.NewInt(0).Add(value, v)
		} else {
			finalRewardInfo[key] = value
		}
	}

	if !check(finalRewardInfo, big.NewInt(0).Add(rnethTotalAmount, nethTotalAmount)) {
		return fmt.Errorf("final reward check failed")
	}

	err = writeJson(nethRewardInfo, "neth", outputDir)
	if err != nil {
		return err
	}
	err = writeJson(rnethRewardInfo, "rneth", outputDir)
	if err != nil {
		return err
	}
	err = writeJson(finalRewardInfo, "final", outputDir)
	if err != nil {
		return err
	}

	return nil
}

func check(rewards map[common.Address]*big.Int, totalAmount *big.Int) bool {
	sum := big.NewInt(0)
	for _, reward := range rewards {
		sum = big.NewInt(0).Add(sum, reward)
	}

	if !(big.NewInt(0).Sub(sum, totalAmount).Uint64() == 0) {
		log.Errorw("check", "sum", sum.String(), "totalAmount", totalAmount.String())
		return false
	}

	return true
}

func writeJson(rewards map[common.Address]*big.Int, name, dir string) error {
	rewardStr := map[string]string{}
	for key, value := range rewards {
		rewardStr[key.String()] = value.String()
	}

	t := time.Now().Format("2006-01-02T15:04:05")
	path := filepath.Join(dir, name+"-reward-"+t+".json")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w (path: %s)", err, path)
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(rewardStr); err != nil {
		return fmt.Errorf("failed to encode total rewards: %w", err)
	}

	return nil
}

func distribute(points map[string]string, totalAmount *big.Int) (map[common.Address]*big.Int, error) {
	pointInfo := map[common.Address]*big.Int{}

	totalPoints := big.NewInt(0)
	for key, value := range points {
		addr := common.HexToAddress(key)
		point, isOk := big.NewInt(0).SetString(value, 10)
		if !isOk {
			return nil, fmt.Errorf("amount parsing failed")
		}
		pointInfo[addr] = point
		totalPoints = big.NewInt(0).Add(totalPoints, point)
	}

	length := len(pointInfo)
	i := 0
	totalAssigned := big.NewInt(0)
	rewardInfo := map[common.Address]*big.Int{}
	for addr, point := range pointInfo {
		if i == length-1 {
			reward := big.NewInt(0).Sub(totalAmount, totalAssigned)
			rewardInfo[addr] = reward
			break
		}
		reward := big.NewInt(0).Div(big.NewInt(0).Mul(totalAmount, point), totalPoints)
		totalAssigned = big.NewInt(0).Add(totalAssigned, reward)
		rewardInfo[addr] = reward
		i++
	}

	return rewardInfo, nil
}

func getPoints(filePath string) (map[string]string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	points := make(map[string]string, 0)
	err = json.Unmarshal(data, &points)
	if err != nil {
		return nil, err
	}

	return points, nil
}
