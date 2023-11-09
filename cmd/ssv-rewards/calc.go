package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"time"

	"github.com/aquasecurity/table"
	"github.com/gocarina/gocsv"
	"github.com/volatiletech/sqlboiler/v4/queries"
	"go.uber.org/zap"
	"golang.org/x/exp/maps"
)

const (
	MonthTimeFormat = "2006-01"
)

type CalcCmd struct {
	Dir                      string `default:"./rewards" help:"Path to save the rewards to,"`
	From                     string `                    help:"From month, for example: 2023-10"                       required:""`
	To                       string `                    help:"To month (inclusive), for example: 2023-11"             required:""`
	PerformanceProvider      string `default:"beaconcha" help:"Performance provider to use."                                       enum:"beaconcha,e2m"`
	MinimumDailyAttestations int    `default:"202"       help:"Minimum attestations in a day to be considered active."`

	db *sql.DB
}

func (c *CalcCmd) Run(logger *zap.Logger, globals *Globals) error {
	ctx := context.Background()

	// Parse from and to months.
	from, err := time.ParseInLocation(MonthTimeFormat, c.From, time.UTC)
	if err != nil {
		return fmt.Errorf("failed to parse from month: %w", err)
	}
	to, err := time.ParseInLocation(MonthTimeFormat, c.To, time.UTC)
	if err != nil {
		return fmt.Errorf("failed to parse to month: %w", err)
	}
	if from.After(to) {
		return fmt.Errorf("from month must be before to month")
	}
	logger.Info(
		"Calculating rewards",
		zap.Int("months", int(math.Ceil(to.Sub(from).Hours()/24/30))),
		zap.Time("from", from),
		zap.Time("to", to),
	)

	// Connect to the PostgreSQL database.
	c.db, err = sql.Open("postgres", globals.Postgres)
	if err != nil {
		return err
	}
	logger.Info("Connected to PostgreSQL")

	// Apply rewards.sql to create/replace database functions.
	rewardsSQL, err := os.ReadFile("rewards.sql")
	if err != nil {
		return fmt.Errorf("failed to read rewards.sql: %w", err)
	}
	if _, err := c.db.ExecContext(ctx, string(rewardsSQL)); err != nil {
		return fmt.Errorf("failed to execute rewards.sql: %w", err)
	}

	// Export rewards for each month.
	type rewardTier struct {
		Month       time.Time
		Days        int
		Validators  int
		DailyReward float64
	}
	var tiers []rewardTier

	type monthlyOwnerReward struct {
		Month string
		OwnerReward
	}
	type totalOwnerReward struct {
		OwnerAddress     string
		TotalAccruedDays int
		TotalSSVReward   float64
	}
	var allOwnerRewards []monthlyOwnerReward
	var totalOwnerRewards = map[string]*totalOwnerReward{}

	type monthlyValidatorReward struct {
		Month string
		ValidatorReward
	}
	var allValidatorRewards []monthlyValidatorReward
	var totalValidatorRewards = map[string]*ValidatorReward{}

	for month := from; month.Before(to.AddDate(0, 1, 0)); month = month.AddDate(0, 1, 0) {
		dir := filepath.Join(c.Dir, month.Format(MonthTimeFormat))
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %q: %w", dir, err)
		}

		ownerRewards, err := c.monthlyOwnerRewards(ctx, month)
		if err != nil {
			return fmt.Errorf("failed to calculate owner rewards: %w", err)
		}
		if err := exportCSV(ownerRewards, filepath.Join(dir, "by-owner.csv")); err != nil {
			return fmt.Errorf("failed to export owner rewards: %w", err)
		}
		for _, ownerReward := range ownerRewards {
			allOwnerRewards = append(allOwnerRewards, monthlyOwnerReward{
				Month:       month.Format(MonthTimeFormat),
				OwnerReward: ownerReward,
			})
			if total, ok := totalOwnerRewards[ownerReward.OwnerAddress]; ok {
				total.TotalAccruedDays += ownerReward.TotalAccruedDays
				total.TotalSSVReward += ownerReward.TotalSSVReward
			} else {
				totalOwnerRewards[ownerReward.OwnerAddress] = &totalOwnerReward{
					OwnerAddress:     ownerReward.OwnerAddress,
					TotalAccruedDays: ownerReward.TotalAccruedDays,
					TotalSSVReward:   ownerReward.TotalSSVReward,
				}
			}
		}

		validatorRewards, err := c.monthlyValidatorRewards(ctx, month)
		if err != nil {
			return fmt.Errorf("failed to calculate validator rewards: %w", err)
		}
		if err := exportCSV(validatorRewards, filepath.Join(dir, "by-validator.csv")); err != nil {
			return fmt.Errorf("failed to export validator rewards: %w", err)
		}
		for _, validatorReward := range validatorRewards {
			allValidatorRewards = append(allValidatorRewards, monthlyValidatorReward{
				Month:           month.Format(MonthTimeFormat),
				ValidatorReward: validatorReward,
			})
			if total, ok := totalValidatorRewards[validatorReward.PublicKey]; ok {
				total.AccruedDays += validatorReward.AccruedDays
				total.SSVReward += validatorReward.SSVReward
			} else {
				totalValidatorRewards[validatorReward.PublicKey] = &ValidatorReward{
					OwnerAddress: validatorReward.OwnerAddress,
					PublicKey:    validatorReward.PublicKey,
					AccruedDays:  validatorReward.AccruedDays,
					SSVReward:    validatorReward.SSVReward,
				}
			}
		}

		// Get tier tier.
		var tier struct {
			DailyReward float64 `boil:"get_tier_reward"`
			DaysInMonth int     `boil:"get_days_in_month"`
		}
		if err := queries.Raw("SELECT * FROM get_tier_reward($1, $2)", c.PerformanceProvider, month).Bind(ctx, c.db, &tier); err != nil {
			return fmt.Errorf("failed to get tier reward: %w", err)
		}
		if err := queries.Raw("SELECT * FROM get_days_in_month($1)", month).Bind(ctx, c.db, &tier); err != nil {
			return fmt.Errorf("failed to get days in month: %w", err)
		}
		tiers = append(tiers, rewardTier{month, tier.DaysInMonth, len(validatorRewards), tier.DailyReward})

		// Export merkle tree for this month.
		totalRewards := map[string]*big.Int{}
		for _, ownerReward := range totalOwnerRewards {
			totalRewards[ownerReward.OwnerAddress], _ = new(big.Float).Mul(
				big.NewFloat(ownerReward.TotalSSVReward),
				big.NewFloat(math.Pow10(18)),
			).Int(nil)
		}
		f, err := os.Create(filepath.Join(dir, "merkle-tree.json"))
		if err != nil {
			return fmt.Errorf("failed to create merkle-tree.csv: %w", err)
		}
		defer f.Close()
		if err := json.NewEncoder(f).Encode(totalRewards); err != nil {
			return fmt.Errorf("failed to encode total rewards: %w", err)
		}
	}

	// Print rewards tiers.
	totalRewards := map[string]float64{}
	for _, ownerReward := range allOwnerRewards {
		totalRewards[ownerReward.Month] += ownerReward.TotalSSVReward
	}
	fmt.Println()
	fmt.Println("Summary")
	tbl := table.New(os.Stdout)
	tbl.SetHeaders("Month", "Eligible Validators", "Tier Reward", "Total Rewards")
	for _, tier := range tiers {
		month := tier.Month.Format(MonthTimeFormat)
		tbl.AddRow(
			fmt.Sprintf("%s (%d days)", month, tier.Days),
			fmt.Sprint(tier.Validators),
			fmt.Sprintf("%v SSV/day", tier.DailyReward),
			fmt.Sprintf("%v SSV", totalRewards[month]),
		)
	}
	tbl.Render()
	fmt.Println()

	// Export concatenated rewards.
	if err := exportCSV(allOwnerRewards, filepath.Join(c.Dir, "by-owner.csv")); err != nil {
		return fmt.Errorf("failed to export owner rewards: %w", err)
	}
	if err := exportCSV(allValidatorRewards, filepath.Join(c.Dir, "by-validator.csv")); err != nil {
		return fmt.Errorf("failed to export validator rewards: %w", err)
	}

	// Export total rewards.
	if err := exportCSV(maps.Values(totalOwnerRewards), filepath.Join(c.Dir, "total-by-owner.csv")); err != nil {
		return fmt.Errorf("failed to export total owner rewards: %w", err)
	}
	if err := exportCSV(maps.Values(totalValidatorRewards), filepath.Join(c.Dir, "total-by-validator.csv")); err != nil {
		return fmt.Errorf("failed to export total validator rewards: %w", err)
	}

	return nil
}

type OwnerReward struct {
	OwnerAddress       string
	NumberOfValidators int
	TotalAccruedDays   int
	TotalSSVReward     float64
}

func (c *CalcCmd) monthlyOwnerRewards(
	ctx context.Context,
	month time.Time,
) (rewards []OwnerReward, err error) {
	return rewards, queries.Raw(
		"SELECT * FROM calculate_final_rewards($1, $2, $3)",
		c.PerformanceProvider, c.MinimumDailyAttestations, month,
	).Bind(ctx, c.db, &rewards)
}

type ValidatorReward struct {
	OwnerAddress string
	PublicKey    string
	AccruedDays  int
	SSVReward    float64
}

func (c *CalcCmd) monthlyValidatorRewards(
	ctx context.Context,
	month time.Time,
) ([]ValidatorReward, error) {
	var rewards []ValidatorReward
	return rewards, queries.Raw(
		"SELECT * FROM calculate_detailed_rewards($1, $2, $3)",
		c.PerformanceProvider, c.MinimumDailyAttestations, month,
	).Bind(ctx, c.db, &rewards)
}

func exportCSV(data any, fileName string) error {
	f, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create %q: %w", fileName, err)
	}
	defer f.Close()
	if err := gocsv.Marshal(data, f); err != nil {
		return fmt.Errorf("failed to marshal %q: %w", fileName, err)
	}
	return nil
}
