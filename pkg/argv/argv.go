package argv

// https://github.com/levicook/argmapper/blob/master/argmapper.go
type Map map[string]string

func ArgsToMap(args []string) (m Map) {
	m = make(Map)
	if len(args) == 0 {
		return
	}

nextopt:
	for i, s := range args {
		// does s look like an option?
		if len(s) > 1 && s[0] == '-' {
			k := ""
			v := ""

			num_minuses := 1
			if s[1] == '-' {
				num_minuses++
			}

			k = s[num_minuses:]
			if len(k) == 0 || k[0] == '-' || k[0] == '=' {
				continue nextopt
			}

			for i := 1; i < len(k); i++ { // equals cannot be first
				if k[i] == '=' {
					v = k[i+1:]
					k = k[0:i]
					break
				}
			}

			// It must have a value, which might be the next arg, assuming the next arg isn't an option too.
			remaining := args[i+1:]
			if v == "" && len(remaining) > 0 && remaining[0][0] != '-' {
				v = remaining[0]
			} // value is the next arg
			m[k] = v
		}
	}
	return m
}

// 获取指定参数名的值,获取失败返回默认值(多个参数名则返回最先找到的值)
func ArgsValue(margs Map, def string, keys ...string) (value string) {
	value = def // 默认值
	for _, key := range keys {
		if v, ok := margs[key]; ok && v != "" { // 找到参数
			value = v // 存储该值
			break
		}
	}
	return value
}
