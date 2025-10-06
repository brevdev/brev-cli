package terminal

import (
	"errors"
	"fmt"
	"os"

	breverrors "github.com/brevdev/brev-cli/pkg/errors"
	"github.com/manifoldco/promptui"
)

type PromptSelectContent struct {
	ErrorMsg string
	Label    string
	Items    []string
}

func PromptSelectInput(pc PromptSelectContent) string {
	// templates := &promptui.SelectTemplates{
	// 	Label:  "{{ . }} ",
	// 	Selected:   "{{ . | green }} ",
	// 	Active: "{{ . | cyan }} ",
	// }

	prompt := promptui.Select{
		Label: pc.Label,
		Items: pc.Items,

		// Templates: templates,
	}

	_, result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}

func DisplayBrevLogo(t *Terminal) { //nolint:funlen // logo
	t.Vprint("Welcome to NVIDIA Brev")
	t.Vprint("")
	t.Vprint("    ##@@@#.")
	t.Vprint("  #@@@@@@@@@@.")
	t.Vprint(" #@@@.   .@@@@%")
	t.Vprint(" #@%       :@@@@-")
	t.Vprint(" #@          =@@@=")
	t.Vprint("#@*           +@@@=            -=%@%=")
	t.Vprint("#@:            =@@@@         *@@@@@@@@")
	t.Vprint("#@:             =@@@@      -@@@=    +@+@@@@@@+")
	t.Vprint("#@:              @@@@%    @@@-       @@:  @@@@#                         @@@@@")
	t.Vprint("#@@=              @@@@+  %@@-               +@@*                     *@@@@@@@@.")
	t.Vprint(" #@@-              @@@@.%@@-                 #@@ .@@@@..           @@@@@=   @@@.")
	t.Vprint(" #@@@=             @@@@@@@@                   @@@@@@@@@@%        *@@@@-      @@#")
	t.Vprint("  #@@@@            -@@@@@@                    -@#      #@@+     @@@@#        @@@")
	t.Vprint("   #@@@%            @@@@@@*                   -@        .@@   =@@@@:         @@@")
	t.Vprint("    #@@@-            @@@@@@          @@@:     -#          @  @@@@*           @@@")
	t.Vprint("     #@@@-           @@@@@@         :@@@@%    -.          @ @@@@             @@#")
	t.Vprint("      #@@@           +@@@@@=        :@@@@@    -           @@@@@             %@@")
	t.Vprint("       #@@@           ..@@@@        =@@@@@    -           @@@@+            .@@-")
	t.Vprint("        #@@:             :@@        @@@@@@                @@@@:           -@@@")
	t.Vprint("         #@@               @        @@@@@@               @@@@@           .@@@")
	t.Vprint("         #@@.              *%       @@@@@:              #+  @@           @@@.")
	t.Vprint("          #@@               @       @@@@#              @    @@          @@@")
	t.Vprint("          #@@-              =       @@@@     @      +@#     @          @@@.")
	t.Vprint("           #@@       .      .       @@@      .     @@@     @@         *@@#")
	t.Vprint("           #@@       @@*     -      @+      #.    -@@@     @@         @@+")
	t.Vprint("           #@@@      @@@:    -             .@     =@@@     @+        @@@")
	t.Vprint("           #@@@      *@@@    -             @@     @@@@     @        %@@")
	t.Vprint("            #@@.     +@@@    -            @@*     @@@@     @       -@@:")
	t.Vprint("            #@@+     .@@@    -           @@@.     :%@              @@%")
	t.Vprint("            #@@@      @@@:   -          .@@@        =.            #@@")
	t.Vprint("            #@@@      @@@    -           #@@                      @@")
	t.Vprint("            #@@@      @@@    -            @#                    .@@+")
	t.Vprint("            #@@@      @@@   .             @:                    @@@")
	t.Vprint("             #@@+      @    +             @:                   +@@:")
	t.Vprint("             #@@@           @             @                    @@@")
	t.Vprint("             #@@@           %      @      %         =         %@@")
	t.Vprint("             #@@@          *%      @      #         @        .@@@")
	t.Vprint("             #@@@.         @      .@     .:        +@        @@@+")
	t.Vprint("              #@@+        :@      .@     #     @..=@@       +@@@-")
	t.Vprint("              #@@@        @=      .@     #     @@@@@@       @@@@")
	t.Vprint("               #@@       *@       @@     @      #@@@*      %@@@@")
	t.Vprint("               #@@#      @@       @-     :        +@*      @@@@")
	t.Vprint("               #@@@.   -@@        @-    ::         %*     *@@@@")
	t.Vprint("                #@@@%%%@@@        @-    :           *     @@@@=")
	t.Vprint("                 #@@@@@@@@       *@-    @           *    @@@@@")
	t.Vprint("                   #@@@@@@       @@     #           @   =@@@@")
	t.Vprint("                    #@@@@@=     .@@    -#           @  =@@@@@")
	t.Vprint("                      #@@@@=    @@@    #            @@@@@@@@")
	t.Vprint("                        #@@@@@@@@@@:  .@            @@@@@@@.")
	t.Vprint("                                  #@  @@           @@@@@@@=")
	t.Vprint("                                   #@@@@#         #@@@@@@=")
	t.Vprint("                                    #@@@@@.     .@@@@@@@")
	t.Vprint("                                      #%@@@@@@@@@@@@@@-")
	t.Vprint("                                          ###@@@@@##")
}

type PromptContent struct {
	ErrorMsg   string
	Label      string
	Default    string
	AllowEmpty bool
	Mask       rune
}

func PromptGetInput(pc PromptContent) string {
	validate := func(input string) error {
		if pc.AllowEmpty {
			return nil
		}
		if len(input) == 0 {
			return breverrors.WrapAndTrace(errors.New(pc.ErrorMsg))
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }} ",
		Valid:   "{{ . | green }} ",
		Invalid: "{{ . | yellow }} ",
		Success: "{{ . | bold }} ",
	}

	prompt := promptui.Prompt{
		Label:     pc.Label,
		Templates: templates,
		Validate:  validate,
		Default:   pc.Default,
		AllowEdit: true,
	}
	if pc.Mask != 0 {
		prompt.Mask = pc.Mask
	}

	result, err := prompt.Run()
	if err != nil {
		fmt.Printf("Prompt failed %v\n", err)
		os.Exit(1)
	}

	return result
}
