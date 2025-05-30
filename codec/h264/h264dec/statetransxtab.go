package h264dec

type StateTransx struct {
	TransIdxLPS, TransIdxMPS int
}

// 9-45
// [pStateIdx]{TransIdxLPS, TransIdxMPS}
var stateTransxTab = map[int]StateTransx{
	0:  {0, 1},
	1:  {0, 2},
	2:  {1, 3},
	3:  {2, 4},
	4:  {2, 5},
	5:  {4, 6},
	6:  {4, 7},
	7:  {5, 8},
	8:  {6, 9},
	9:  {7, 10},
	10: {8, 11},
	11: {9, 12},
	12: {9, 13},
	13: {11, 14},
	14: {11, 15},
	15: {12, 16},
	16: {13, 17},
	17: {13, 18},
	18: {15, 19},
	19: {15, 20},
	20: {16, 21},
	21: {16, 22},
	22: {18, 23},
	23: {18, 24},
	24: {19, 25},
	25: {19, 26},
	26: {21, 27},
	27: {21, 28},
	28: {22, 29},
	29: {22, 30},
	30: {23, 31},
	31: {24, 32},
	32: {24, 33},
	33: {25, 34},
	34: {26, 35},
	35: {26, 36},
	36: {27, 37},
	37: {27, 38},
	38: {28, 39},
	39: {29, 40},
	40: {29, 41},
	41: {30, 42},
	42: {30, 43},
	43: {30, 44},
	44: {31, 45},
	45: {32, 46},
	46: {32, 47},
	47: {33, 48},
	48: {33, 49},
	49: {33, 50},
	50: {34, 51},
	51: {34, 52},
	52: {35, 53},
	53: {35, 54},
	54: {35, 55},
	55: {36, 56},
	56: {36, 57},
	57: {36, 58},
	58: {37, 59},
	59: {37, 61},
	60: {37, 61},
	61: {38, 62},
	62: {38, 62},
	63: {63, 63},
}
