# ssv-rewards

Distribute SSV token rewards based on points

## Installation

```bash
 go build -o ssv-reward cmd/*
```

## Usage

### Calculation

you may calculate the reward distribution:

```bash
./ssv-reward calc --nethPointsInputPath ./data/neth-point.json --rnethPointsInputPath ./data/rneth-point.json --nethSsvRewardAmount 254
000000000000000000 --rnethSsvRewardAmount 556000000000000000000 --outputDir ./data
```

### Merkleization

After calculating the reward distribution, you may merkleize the rewards for a specific round.

1. Copy the file at `./data/final-reward-xxxxxx.json` over to `./scripts/merkle-generator/scripts/input_1.json`.
2. Run the merkleization script:
   ```bash
   cd scripts/merkle-generator
   npm i
   npx hardhat run scripts/merkle.ts
   ```
3. The merkle tree is generated at `./merkle-generator/scripts/output-1.json`.
