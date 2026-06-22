package evaluator

import (
	"math/rand"
	"testing"
)

func BenchmarkEvaluateRange_SetForLife(b *testing.B) {
	// Create mock draws
	r := rand.New(rand.NewSource(42))
	draws := make([]FastDraw, 50)
	for i := range draws {
		// Pick 5 random primary numbers from 1 to 47
		var primaryMask uint64
		for p := 0; p < 5; p++ {
			num := r.Intn(47) + 1
			primaryMask |= (1 << uint(num))
		}
		// Pick 1 random secondary number from 1 to 10
		secondaryMask := uint64(1 << uint(r.Intn(10)+1))

		draws[i] = FastDraw{
			DrawNo:        i + 1,
			PrimaryMask:   primaryMask,
			SecondaryMask: secondaryMask,
		}
		// Populate some prize values
		draws[i].PrizeMatrix[5][1] = 360000000
		draws[i].PrizeMatrix[5][0] = 12000000
		draws[i].PrizeMatrix[4][1] = 25000
		draws[i].PrizeMatrix[4][0] = 5000
		draws[i].PrizeMatrix[3][1] = 3000
		draws[i].PrizeMatrix[3][0] = 2000
		draws[i].PrizeMatrix[2][1] = 1000
		draws[i].PrizeMatrix[2][0] = 500
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Evaluate a chunk of 10,000 combinations
		EvaluateRange(0, 10000, SetForLifeConfig, draws, 5)
	}
}

func BenchmarkEvaluateRange_Lotto(b *testing.B) {
	// Create mock draws
	r := rand.New(rand.NewSource(42))
	draws := make([]FastDraw, 50)
	for i := range draws {
		// Pick 6 random primary numbers from 1 to 59
		var primaryMask uint64
		for p := 0; p < 6; p++ {
			num := r.Intn(59) + 1
			primaryMask |= (1 << uint(num))
		}
		// Pick 1 random secondary number from 1 to 59 (bonus ball)
		secondaryMask := uint64(1 << uint(r.Intn(59)+1))

		draws[i] = FastDraw{
			DrawNo:        i + 1,
			PrimaryMask:   primaryMask,
			SecondaryMask: secondaryMask,
		}
		// Populate some prize values
		draws[i].PrizeMatrix[6][0] = 100000000
		draws[i].PrizeMatrix[5][1] = 5000000
		draws[i].PrizeMatrix[5][0] = 100000
		draws[i].PrizeMatrix[4][0] = 14000
		draws[i].PrizeMatrix[3][0] = 3000
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Evaluate a chunk of 10,000 combinations
		EvaluateRange(0, 10000, LottoConfig, draws, 5)
	}
}

func BenchmarkEvaluateRange_Thunderball(b *testing.B) {
	// Create mock draws
	r := rand.New(rand.NewSource(42))
	draws := make([]FastDraw, 50)
	for i := range draws {
		// Pick 5 random primary numbers from 1 to 39
		var primaryMask uint64
		for p := 0; p < 5; p++ {
			num := r.Intn(39) + 1
			primaryMask |= (1 << uint(num))
		}
		// Pick 1 random secondary number from 1 to 14
		secondaryMask := uint64(1 << uint(r.Intn(14)+1))

		draws[i] = FastDraw{
			DrawNo:        i + 1,
			PrimaryMask:   primaryMask,
			SecondaryMask: secondaryMask,
		}
		// Populate some prize values
		draws[i].PrizeMatrix[5][1] = 50000000 // Thunderball jackpot: £500,000 (50,000,000 pence)
		draws[i].PrizeMatrix[5][0] = 500000
		draws[i].PrizeMatrix[4][1] = 25000
		draws[i].PrizeMatrix[4][0] = 10000
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Evaluate a chunk of 10,000 combinations
		EvaluateRange(0, 10000, ThunderballConfig, draws, 5)
	}
}

func BenchmarkEvaluateRange_EuroMillions(b *testing.B) {
	// Create mock draws
	r := rand.New(rand.NewSource(42))
	draws := make([]FastDraw, 50)
	for i := range draws {
		// Pick 5 random primary numbers from 1 to 50
		var primaryMask uint64
		for p := 0; p < 5; p++ {
			num := r.Intn(50) + 1
			primaryMask |= (1 << uint(num))
		}
		// Pick 2 random secondary numbers from 1 to 12
		var secondaryMask uint64
		for s := 0; s < 2; s++ {
			num := r.Intn(12) + 1
			secondaryMask |= (1 << uint(num))
		}

		draws[i] = FastDraw{
			DrawNo:        i + 1,
			PrimaryMask:   primaryMask,
			SecondaryMask: secondaryMask,
		}
		// Populate some prize values
		draws[i].PrizeMatrix[5][2] = 17000000000 // EuroMillions jackpot: ~£170M (17,000,000,000 pence)
		draws[i].PrizeMatrix[5][1] = 20000000
		draws[i].PrizeMatrix[5][0] = 2000000
		draws[i].PrizeMatrix[4][2] = 160000
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Evaluate a chunk of 10,000 combinations
		EvaluateRange(0, 10000, EuroMillionsConfig, draws, 5)
	}
}

