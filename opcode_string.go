// Code generated by "stringer -type Opcode enum.go"; DO NOT EDIT.

package virtual

import "fmt"

const _Opcode_name = "NopAPAddF32AddF64AddC64AddC128AddI32AddI64AddPtrAddPtrsAddSPAnd16And32And64And8ArgumentArgument16Argument32Argument64Argument8ArgumentsArgumentsFPBPBitfieldI8BitfieldI16BitfieldI32BitfieldI64BitfieldU8BitfieldU16BitfieldU32BitfieldU64BoolC128BoolF32BoolF64BoolI16BoolI32BoolI64BoolI8CallCallFPConvC64C128ConvF32C128ConvF32C64ConvF32F64ConvF32I32ConvF32I64ConvF32U32ConvF64C128ConvF64F32ConvF64I32ConvF64I64ConvF64I8ConvF64U16ConvF64U32ConvF64U64ConvI16I32ConvI16I64ConvI16U32ConvI32C128ConvI32C64ConvI32F32ConvI32F64ConvI32I16ConvI32I64ConvI32I8ConvI64ConvI64F64ConvI64I16ConvI64I32ConvI64I8ConvI64U16ConvI8I16ConvI8I32ConvI8I64ConvI8F64ConvI8U32ConvU16I32ConvU16I64ConvU16U32ConvU16U64ConvU32F32ConvU32F64ConvU32I16ConvU32I64ConvU32U8ConvU8I16ConvU8I32ConvU8U32ConvU8U64CopyCpl32Cpl64Cpl8DSDSC128DSI16DSI32DSI64DSI8DSNDivC128DivC64DivF32DivF64DivI32DivI64DivU32DivU64Dup32Dup64Dup8EqF32EqF64EqI32EqI64EqI8ExtFFIReturnFPField16Field64Field8FuncGeqF32GeqF64GeqI32GeqI64GeqI8GeqU32GeqU64GtF32GtF64GtI32GtI64GtU32GtU64IndexIndexI16IndexU16IndexI32IndexI64IndexI8IndexU32IndexU64IndexU8JmpJmpPJnzJzLabelLeqF32LeqF64LeqI32LeqI64LeqI8LeqU32LeqU64LoadLoad16Load32Load64Load8LshI16LshI32LshI64LshI8LtF32LtF64LtI32LtI64LtU32LtU64MulC128MulC64MulF32MulF64MulI32MulI64NegF32NegF64NegI16NegI32NegI64NegI8NegIndexI32NegIndexI64NegIndexU16NegIndexU32NegIndexU64NeqC128NeqC64NeqF32NeqF64NeqI32NeqI64NeqI8NotOr32Or64PanicPostIncF64PostIncI16PostIncI32PostIncI64PostIncI8PostIncPtrPostIncU32BitsPostIncU64BitsPreIncI16PreIncI32PreIncI64PreIncI8PreIncPtrPreIncU32BitsPreIncU64BitsPtrDiffPush16Push32Push64Push8PushC128RemI32RemI64RemU32RemU64ReturnRshI16RshI32RshI64RshI8RshU16RshU32RshU64RshU8StoreStore16Store32Store64Store8StoreBits16StoreBits32StoreBits64StoreBits8StoreC128StrNCopySubF32SubF64SubI32SubI64SubPtrsSwitchI32SwitchI64TextVariableVariable16Variable32Variable64Variable8Xor32Xor64Zero8Zero16Zero32Zero64__assert_fail__ctype_b_loc__signbit__signbitfabortabsaccessacosallocaasinatanatoibswap64builtincallocceilcimagfclose_clrsbclrsblclrsbllclzclzlclzllcopysigncoscoshcrealfctzctzlctzlldlclosedlerrordlopendlsymerrno_locationexitexpfabsfchmodfchownfcloseferrorfcntlfflushffsffslffsllfgetcfgetsfloorfopen64fprintfframeAddressfreadfreefseekfstat64fsyncftellftruncate64fwritegetcwdgetenvgeteuidgetpidgettimeofdayisinfisinffisinflisprintlocaltimeloglog10longjmplseek64lstat64mallocmalloc_usable_sizememcmpmemcpymemmovemempcpymemsetmkdirmmap64munmapopen64parityparitylparityllpopcountpopcountlpopcountllpowprintfputspthread_cond_signalpthread_cond_waitpthread_createpthread_detachpthread_equalpthread_joinpthread_mutex_destroypthread_mutex_initpthread_mutex_lockpthread_mutex_trylockpthread_mutex_unlockpthread_mutexattr_destroypthread_mutexattr_initpthread_mutexattr_settypepthread_selfqsortreadreadlinkreallocrecvregister_stdfilesreturnAddressrewindrmdirroundsched_yieldselect_setjmpsinsinhsleepsprintfsqrtstat64strcatstrchrstrcmpstrcpystrerror_rstrlenstrncmpstrncpystrrchrsysconfsystemtantanhtimetolowerunlinkusleeputimesvfprintfvprintfwrite_beginthreadex_endthreadex_msizeAreFileApisANSICloseHandleCreateFileMappingACreateFileMappingWCreateMutexWCreateFileACreateFileWDeleteCriticalSectionDeleteFileADeleteFileWEnterCriticalSectionFlushFileBuffersFlushViewOfFileFormatMessageAFormatMessageWFreeLibraryGetCurrentProcessIdGetCurrentThreadIdGetDiskFreeSpaceAGetDiskFreeSpaceWGetFileAttributesAGetFileAttributesWGetFileAttributesExWGetFileSizeGetFullPathNameAGetFullPathNameWGetLastErrorGetProcAddressGetProcessHeapGetSystemInfoGetSystemTimeGetSystemTimeAsFileTimeGetTempPathAGetTempPathWGetTickCountGetVersionExAGetVersionExWHeapAllocHeapCreateHeapCompactHeapDestroyHeapFreeHeapReAllocHeapSizeHeapValidateInitializeCriticalSectionInterlockedCompareExchangeLoadLibraryALoadLibraryWLocalFreeLockFileLockFileExLeaveCriticalSectionMapViewOfFileMultiByteToWideCharOutputDebugStringAOutputDebugStringWQueryPerformanceCounterReadFileSetEndOfFileSetFilePointerSleepSystemTimeToFileTimeUnlockFileUnlockFileExUnmapViewOfFileWaitForSingleObjectWaitForSingleObjectExWideCharToMultiByteWriteFile"

var _Opcode_index = [...]uint16{0, 3, 5, 11, 17, 23, 30, 36, 42, 48, 55, 60, 65, 70, 75, 79, 87, 97, 107, 117, 126, 135, 146, 148, 158, 169, 180, 191, 201, 212, 223, 234, 242, 249, 256, 263, 270, 277, 283, 287, 293, 304, 315, 325, 335, 345, 355, 365, 376, 386, 396, 406, 415, 425, 435, 445, 455, 465, 475, 486, 496, 506, 516, 526, 536, 545, 552, 562, 572, 582, 591, 601, 610, 619, 628, 637, 646, 656, 666, 676, 686, 696, 706, 716, 726, 735, 744, 753, 762, 771, 775, 780, 785, 789, 791, 797, 802, 807, 812, 816, 819, 826, 832, 838, 844, 850, 856, 862, 868, 873, 878, 882, 887, 892, 897, 902, 906, 909, 918, 920, 927, 934, 940, 944, 950, 956, 962, 968, 973, 979, 985, 990, 995, 1000, 1005, 1010, 1015, 1020, 1028, 1036, 1044, 1052, 1059, 1067, 1075, 1082, 1085, 1089, 1092, 1094, 1099, 1105, 1111, 1117, 1123, 1128, 1134, 1140, 1144, 1150, 1156, 1162, 1167, 1173, 1179, 1185, 1190, 1195, 1200, 1205, 1210, 1215, 1220, 1227, 1233, 1239, 1245, 1251, 1257, 1263, 1269, 1275, 1281, 1287, 1292, 1303, 1314, 1325, 1336, 1347, 1354, 1360, 1366, 1372, 1378, 1384, 1389, 1392, 1396, 1400, 1405, 1415, 1425, 1435, 1445, 1454, 1464, 1478, 1492, 1501, 1510, 1519, 1527, 1536, 1549, 1562, 1569, 1575, 1581, 1587, 1592, 1600, 1606, 1612, 1618, 1624, 1630, 1636, 1642, 1648, 1653, 1659, 1665, 1671, 1676, 1681, 1688, 1695, 1702, 1708, 1719, 1730, 1741, 1751, 1760, 1768, 1774, 1780, 1786, 1792, 1799, 1808, 1817, 1821, 1829, 1839, 1849, 1859, 1868, 1873, 1878, 1883, 1889, 1895, 1901, 1914, 1927, 1936, 1946, 1951, 1954, 1960, 1964, 1970, 1974, 1978, 1982, 1989, 1996, 2002, 2006, 2012, 2018, 2023, 2029, 2036, 2039, 2043, 2048, 2056, 2059, 2063, 2069, 2072, 2076, 2081, 2088, 2095, 2101, 2106, 2120, 2124, 2127, 2131, 2137, 2143, 2149, 2155, 2160, 2166, 2169, 2173, 2178, 2183, 2188, 2193, 2200, 2207, 2219, 2224, 2228, 2233, 2240, 2245, 2250, 2261, 2267, 2273, 2279, 2286, 2292, 2304, 2309, 2315, 2321, 2328, 2337, 2340, 2345, 2352, 2359, 2366, 2372, 2390, 2396, 2402, 2409, 2416, 2422, 2427, 2433, 2439, 2445, 2451, 2458, 2466, 2474, 2483, 2493, 2496, 2502, 2506, 2525, 2542, 2556, 2570, 2583, 2595, 2616, 2634, 2652, 2673, 2693, 2718, 2740, 2765, 2777, 2782, 2786, 2794, 2801, 2805, 2822, 2835, 2841, 2846, 2851, 2862, 2869, 2875, 2878, 2882, 2887, 2894, 2898, 2904, 2910, 2916, 2922, 2928, 2938, 2944, 2951, 2958, 2965, 2972, 2978, 2981, 2985, 2989, 2996, 3002, 3008, 3014, 3022, 3029, 3034, 3048, 3060, 3066, 3081, 3092, 3110, 3128, 3140, 3151, 3162, 3183, 3194, 3205, 3225, 3241, 3256, 3270, 3284, 3295, 3314, 3332, 3349, 3366, 3384, 3402, 3422, 3433, 3449, 3465, 3477, 3491, 3505, 3518, 3531, 3554, 3566, 3578, 3590, 3603, 3616, 3625, 3635, 3646, 3657, 3665, 3676, 3684, 3696, 3721, 3747, 3759, 3771, 3780, 3788, 3798, 3818, 3831, 3850, 3868, 3886, 3909, 3917, 3929, 3943, 3948, 3968, 3978, 3990, 4005, 4024, 4045, 4064, 4073}

func (i Opcode) String() string {
	if i < 0 || i >= Opcode(len(_Opcode_index)-1) {
		return fmt.Sprintf("Opcode(%d)", i)
	}
	return _Opcode_name[_Opcode_index[i]:_Opcode_index[i+1]]
}
