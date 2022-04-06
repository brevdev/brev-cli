package importpkg

// import "strings"

// type ShellFragment struct {
// 	Name *string `json:"name"`
// 	Tag *string `json:"tag"`
// 	Comment *string `json:"comment"`
// 	Script *[]string `json:"script"`
// 	Dependencies *[]string `json:"dependencies"`
// }

// // def parse_comment_line(line)
// //     comment_free_line = line.gsub("#", "").strip
// //     parts = comment_free_line.split(" ")
// //     return {"dependencies" => parts[1..]} if parts[0].start_with?("dependencies")
// //     parts.length <= 2 ? parts : comment_free_line
// // end

// func parse_comment_line(line string) *ShellFragment {
//     comment_free_line := strings.Replace(line, "#", "", -1)
//     parts := strings.Split(comment_free_line, " ")

// 	if strings.HasPrefix(parts[0], "dependencies") {
// 		temp := parts[1:]
// 		return &ShellFragment{Dependencies : &temp}
// 	}
// 	// map[string][]string
// 	if len(parts) ==2 {
// 		return &ShellFragment{Name: &parts[0], Tag: &parts[1]}
// 	} else if len(parts) ==1 {
// 		return &ShellFragment{Name: &parts[0]}
// 	} else {
// 		return &ShellFragment{Comment: &comment_free_line}
// 	}
// }

// type temptype struct {
// 	CurrentFragment ShellFragment
// 	ScriptSoFar []ShellFragment
// }

// // func from_sh(sh_script string) []ShellFragment {
// //     lines := strings.Split(sh_script, "\n")
// // 	var acc temptype
// // 	for _, line := range lines {
// // 		if strings.HasPrefix(line, "#") {
// // 			if len(*acc.CurrentFragment.Script) == 0 {
// // 				acc.ScriptSoFar = append(acc.ScriptSoFar, acc.CurrentFragment)
// // 				acc.CurrentFragment = ShellFragment{}
// // 			}
// // 			acc.CurrentFragment = *parse_comment_line(line)
// // 		}
// // 	}
// // //     val = lines.reduce([{"name" => nil, "tag" => nil, "comment" =>nil, "script" => []}, []]) do |acc, line|
// // //         current_fragment, script_so_far = acc
// // //         if line.start_with?("#")
// // //             if !current_fragment['script'].empty?
// // //                 script_so_far.push(current_fragment)
// // //                 current_fragment = {"name" => nil, "tag" => nil, "comment" =>nil, "script" => []}
// // //             end
// // //             parsed = parse_comment_line(line)
// // //             if parsed.kind_of?(Array) ## then it has the form [name, tag]
// // //                 current_fragment['name']=parsed[0]
// // //                 current_fragment['tag']=parsed[1]
// // //             elsif parsed.kind_of?(Hash) ## then it has the form dependencies => [...]
// // //                 current_fragment.merge!(parsed)
// // //             else ## otherwise it is a comment string
// // //                 current_fragment['comment'] = parsed
// // //             end
// // //         else
// // //             current_fragment["script"].push(line)
// // //         end
// // //         [current_fragment, script_so_far]
// // //     end
// // //     last_frag, script_so_far = val
// // //     script_so_far + [last_frag]
// // // end
// // }
