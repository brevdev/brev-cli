#!/usr/bin/ruby
#get dotted keypath per person, with project value as default, and foldl across the split keypath -- at each stage getting info for the user/org in general, and per-project/branch (or per-project if current branch unspecified by that user/org). In reality, this is per-workspace, with the ability to attach a workspace as a variant to a project if so desired (but with the ability to keep them free as well). this are aggregated as a pair of lists. at the end the lists are concatenated and interpreted.
#
#there's a 'compact' function that reads left-to-right, storing a dictionary of elements it has seen so far, and the compacted list - if it sees an element it has seen before, if it is overwrite it overwrites the entry in the dictionary of elements - if it's append it discards it. if it isn't tagged, then overwrite removes the dictionary and the list-so-far, and append puts it at the end. prepend places it at the beginning.
#
#there is likewise an interpret function that reads left-to-right and writes out the desired brev file. [[it calls compact and then it folds left with flip cons . get 'sh']]
#
#add # dependencies: so they can be declared
#add overwrite/merge logic
#add cycle-checking to the dependencies

def sh_fragment(opts={})
  {"name" => nil, "tag" => nil, "comment" =>nil, "script" => [], "dependencies" => nil}.merge(opts)
end

KNOWN_SHELL_FRAGMENTS = {
   "c" => sh_fragment("tag" => "c", "name" => "c", "dependencies" => ["machine"], "comment" => "this is the system default c installation", "script" => ["apt-get install c"]),
    "machine" => sh_fragment("tag" => "machine", "name" => "machine", "comment" => "this is the system default machine installation", "script" => ["apt-get install machine"])
}

def parse_comment_line(line)
    comment_free_line = line.gsub("#", "").strip
    parts = comment_free_line.split(" ")
    return {"dependencies" => parts[1..]} if parts[0].start_with?("dependencies")
    parts.length <= 2 ? parts : comment_free_line
end

def from_sh(sh_script)
    lines = sh_script.split("\n")
    val = lines.reduce([{"name" => nil, "tag" => nil, "comment" =>nil, "script" => []}, []]) do |acc, line|
        current_fragment, script_so_far = acc
        if line.start_with?("#") 
            if !current_fragment['script'].empty?
                script_so_far.push(current_fragment)
                current_fragment = {"name" => nil, "tag" => nil, "comment" =>nil, "script" => []}
            end
            parsed = parse_comment_line(line)
            if parsed.kind_of?(Array) ## then it has the form [name, tag]
                current_fragment['name']=parsed[0]
                current_fragment['tag']=parsed[1]
            elsif parsed.kind_of?(Hash) ## then it has the form dependencies => [...]
                current_fragment.merge!(parsed)
            else ## otherwise it is a comment string
                current_fragment['comment'] = parsed
            end
        else
            current_fragment["script"].push(line)
        end
        [current_fragment, script_so_far]
    end
    last_frag, script_so_far = val
    script_so_far + [last_frag]
end


def import(path_to_brev)
    splits = path_to_brev.split("-")
    newpath = (splits.length == 1 ? splits+splits : splits).join("/")+".sh" 
    puts "PATH TO READ IS #{newpath}"
    f = File.read(newpath)
    from_sh(f)
end

def to_sh(script)
    script.flat_map{|x| ["# #{x['name']} #{x['tag']}", "# #{x['comment']}", x['script'].join("\n")]}.join("\n")
end

def prepend(shoriginal, shnew)
    shnew+shoriginal
end

def append(shoriginal, shnew)
    shoriginal+shnew
end

def overwrite(shoriginal, shnew)
    shnew
end

def find_dependencies(sh_name, baseline_dependencies, global_dependencies)
    definition = (baseline_dependencies[sh_name]||global_dependencies[sh_name])||{}
    definition_dependencies = definition['dependencies']||[]
    definition_dependencies.flat_map{|dep_name|  find_dependencies(dep_name, baseline_dependencies, global_dependencies) + [dep_name]}
end

def prepend_dependencies(sh_name, baseline_dependencies, global_dependencies=KNOWN_SHELL_FRAGMENTS)
    dependencies = find_dependencies(sh_name, baseline_dependencies, global_dependencies).uniq
    baseline_deps = dependencies.filter {|dep_name| baseline_dependencies[dep_name]}
    non_baseline_dependencies = dependencies.filter {|dep_name| !baseline_dependencies[dep_name]}
    can_be_fixed_dependencies = non_baseline_dependencies.filter{|dep_name| global_dependencies[dep_name]}
    cant_be_fixed_dependencies = non_baseline_dependencies - can_be_fixed_dependencies
    [can_be_fixed_dependencies.reduce({
        sh_fragment['name'] => sh_fragment
    }) do |deps, dep_name|
        deps[dep_name] = global_dependencies[dep_name]
        deps
    end.merge(baseline_dependencies), dependencies + [sh_name], cant_be_fixed_dependencies]
end

def to_defs_dict(sh_fragment)
    sh_fragment.reduce({}) {|acc, x| acc[x['name']]=x; acc}
end

def split_into_library_and_seq(sh_fragment)
    [sh_fragment.map{|x| x['name'] }, to_defs_dict(sh_fragment)]
end

def add_dependencies(sh_fragment, baseline_dependencies={}, global_dependencies=KNOWN_SHELL_FRAGMENTS)
    ## it's a left fold across the import statements
    ## at any point, if the dependencies aren't already in the loaded dictionary
    ## we add them before we add the current item, and we add them and the current item into the loaded dictionary
    order, shell_defs, failures = split_into_library_and_seq(sh_fragment)
    new_shell_defs, new_order, new_failures = order.reduce([shell_defs, [], []]) do |acc, sh_name|
        shell_defs, newer_order, new_failures = acc
        new_shell_defs, newest_order, newest_failures = prepend_dependencies(sh_name, shell_defs, global_dependencies)
        [new_shell_defs, newer_order + newest_order + [sh_name], new_failures + newest_failures]
    end
    [new_shell_defs, new_order.uniq, new_failures]
end

def process(file)
    shell_defs, order, failures = add_dependencies(file)
    puts "FAILED TO FIND INSTALLATION INSTRUCTIONS FOR: #{failures}" if failures.length > 0
    order.map{|x| shell_defs[x]}
end

def merge(*files)
    to_sh(process(files.flat_map {|f| import f}))
end
puts "outputting combination of shell scripts #{ARGV.join(', ')} to .brev"
File.write('setup.sh', merge(*ARGV))

