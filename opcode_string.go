// Code generated by "stringer -type Opcode enum.go"; DO NOT EDIT

package virtual

import "fmt"

const _Opcode_name = "NopAPAddF32AddF64AddI32AddI64AddPtrAddPtrsAddSPAnd16And32And64And8ArgumentArgument16Argument32Argument64Argument8ArgumentsArgumentsFPBPBitfieldI8BitfieldI16BitfieldI32BitfieldI64BitfieldU8BitfieldU16BitfieldU32BitfieldU64BoolC128BoolF32BoolF64BoolI16BoolI32BoolI64BoolI8CallCallFPConvC64C128ConvF32F64ConvF32I32ConvF32U32ConvF64F32ConvF64I32ConvF64I64ConvF64I8ConvF64U16ConvF64U32ConvF64U64ConvI16I32ConvI16I64ConvI16U32ConvI32C128ConvI32C64ConvI32F32ConvI32F64ConvI32I16ConvI32I64ConvI32I8ConvI64F64ConvI64I16ConvI64I32ConvI64I8ConvI64U16ConvI8I16ConvI8I32ConvI8I64ConvI8U32ConvU16I32ConvU16I64ConvU16U32ConvU32F32ConvU32F64ConvU32I16ConvU32I64ConvU32U8ConvU8I16ConvU8I32ConvU8U32ConvU8U64CopyCpl32Cpl64Cpl8DSDSC128DSI16DSI32DSI64DSI8DSNDivF32DivF64DivI32DivI64DivU32DivU64Dup32Dup64Dup8EqF32EqF64EqI32EqI64EqI8ExtFPFuncGeqF32GeqF64GeqI32GeqI64GeqU32GeqU64GtF32GtF64GtI32GtI64GtU32GtU64IndexIndexI16IndexI32IndexI64IndexU32IndexU64IndexU8JmpJmpPJnzJzLabelLeqF32LeqF64LeqI32LeqI64LeqU32LeqU64LoadLoad16Load32Load64Load8LshI16LshI32LshI64LshI8LtF32LtF64LtI32LtI64LtU32LtU64MulC64MulF32MulF64MulI32MulI64NegF32NegF64NegI16NegI32NegI64NegI8NegIndexI32NegIndexI64NegIndexU64NeqC128NeqC64NeqF32NeqF64NeqI32NeqI64NotOr32Or64PanicPostIncF64PostIncI16PostIncI32PostIncI64PostIncI8PostIncPtrPostIncU32BitsPostIncU64BitsPreIncI16PreIncI32PreIncI64PreIncI8PreIncPtrPreIncU32BitsPreIncU64BitsPtrDiffPush8Push16Push32Push64RemI32RemI64RemU32RemU64ReturnRshI16RshI32RshI64RshI8RshU16RshU32RshU64RshU8StoreStore16Store32Store64Store8StoreBits16StoreBits32StoreBits64StoreBits8StrNCopySubF32SubF64SubI32SubI64SubPtrsTextVariableVariable16Variable32Variable64Variable8Xor32Xor64Zero8Zero16Zero32Zero64abortabsacosallocaasinatancallocceilclrsbclrsblclrsbllclzclzlclzllcoscoshctzctzlctzllexitexpfabsfcloseffsffslffsllfgetcfgetsfloorfopenfprintffreadfreefwriteisinfisinffisinflisprintloglog10mallocmemcmpmemcpymemsetparityparitylparityllpopcountpopcountlpopcountllpowprintfreturnAddressroundsinsinhsprintfsqrtstrcatstrchrstrcmpstrcpystrlenstrncmpstrncpystrrchrtantanhtolowervfprintfvprintf"

var _Opcode_index = [...]uint16{0, 3, 5, 11, 17, 23, 29, 35, 42, 47, 52, 57, 62, 66, 74, 84, 94, 104, 113, 122, 133, 135, 145, 156, 167, 178, 188, 199, 210, 221, 229, 236, 243, 250, 257, 264, 270, 274, 280, 291, 301, 311, 321, 331, 341, 351, 360, 370, 380, 390, 400, 410, 420, 431, 441, 451, 461, 471, 481, 490, 500, 510, 520, 529, 539, 548, 557, 566, 575, 585, 595, 605, 615, 625, 635, 645, 654, 663, 672, 681, 690, 694, 699, 704, 708, 710, 716, 721, 726, 731, 735, 738, 744, 750, 756, 762, 768, 774, 779, 784, 788, 793, 798, 803, 808, 812, 815, 817, 821, 827, 833, 839, 845, 851, 857, 862, 867, 872, 877, 882, 887, 892, 900, 908, 916, 924, 932, 939, 942, 946, 949, 951, 956, 962, 968, 974, 980, 986, 992, 996, 1002, 1008, 1014, 1019, 1025, 1031, 1037, 1042, 1047, 1052, 1057, 1062, 1067, 1072, 1078, 1084, 1090, 1096, 1102, 1108, 1114, 1120, 1126, 1132, 1137, 1148, 1159, 1170, 1177, 1183, 1189, 1195, 1201, 1207, 1210, 1214, 1218, 1223, 1233, 1243, 1253, 1263, 1272, 1282, 1296, 1310, 1319, 1328, 1337, 1345, 1354, 1367, 1380, 1387, 1392, 1398, 1404, 1410, 1416, 1422, 1428, 1434, 1440, 1446, 1452, 1458, 1463, 1469, 1475, 1481, 1486, 1491, 1498, 1505, 1512, 1518, 1529, 1540, 1551, 1561, 1569, 1575, 1581, 1587, 1593, 1600, 1604, 1612, 1622, 1632, 1642, 1651, 1656, 1661, 1666, 1672, 1678, 1684, 1689, 1692, 1696, 1702, 1706, 1710, 1716, 1720, 1725, 1731, 1738, 1741, 1745, 1750, 1753, 1757, 1760, 1764, 1769, 1773, 1776, 1780, 1786, 1789, 1793, 1798, 1803, 1808, 1813, 1818, 1825, 1830, 1834, 1840, 1845, 1851, 1857, 1864, 1867, 1872, 1878, 1884, 1890, 1896, 1902, 1909, 1917, 1925, 1934, 1944, 1947, 1953, 1966, 1971, 1974, 1978, 1985, 1989, 1995, 2001, 2007, 2013, 2019, 2026, 2033, 2040, 2043, 2047, 2054, 2062, 2069}

func (i Opcode) String() string {
	if i < 0 || i >= Opcode(len(_Opcode_index)-1) {
		return fmt.Sprintf("Opcode(%d)", i)
	}
	return _Opcode_name[_Opcode_index[i]:_Opcode_index[i+1]]
}
