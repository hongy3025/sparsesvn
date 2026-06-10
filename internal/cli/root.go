package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/hongy3025/sparsesvn/internal/config"
	"github.com/hongy3025/sparsesvn/internal/svn"
	"github.com/spf13/cobra"
)

type GlobalFlags struct {
	ConfigFile      string
	Workdir         string
	WorkdirExplicit bool   // 标记 -C 是否为用户显式指定
	Verbose         int
	Quiet           bool
	JSON            bool
	NoColor         bool
	ResolvedURL     string // 解析后的最终 URL
}

type exitError struct {
	Code int
	Err  error
}

func (e *exitError) Error() string { return e.Err.Error() }

func newRootCmd(version string) *cobra.Command {
	var verbose int
	gf := &GlobalFlags{}

	cmd := &cobra.Command{
		Use:     "sparsesvn",
		Short:   "Sparse SVN working copy management tool",
		Version: version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:    cobra.NoArgs,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			gf.ConfigFile, _ = cmd.Flags().GetString("file")
			gf.Workdir, _ = cmd.Flags().GetString("workdir")
			gf.Verbose = countVerbose(cmd)
			gf.Quiet, _ = cmd.Flags().GetBool("quiet")
			gf.JSON, _ = cmd.Flags().GetBool("json")
			gf.NoColor, _ = cmd.Flags().GetBool("no-color")

			// 检测 -C 是否为显式指定
			gf.WorkdirExplicit = cmd.Flags().Changed("workdir")

			// 执行智能检测（validate 子命令跳过，因为它不需要 workdir）
			if cmd.Name() != "validate" && cmd.Parent() != nil {
				if err := validateAndDisplayContext(gf, cmd.ErrOrStderr()); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), "Error:", err)
					return &exitError{Code: 2, Err: err}
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.PersistentFlags().StringP("file", "f", "./sparsesvn.yaml", "config file path")
	cmd.PersistentFlags().StringP("workdir", "C", ".", "working directory")
	cmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "verbose output (can be specified multiple times)")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "suppress output")
	cmd.PersistentFlags().Bool("json", false, "output in JSON format")
	cmd.PersistentFlags().Bool("no-color", false, "disable colored output")

	cmd.AddCommand(newValidateCmd(gf))
	cmd.AddCommand(newPlanCmd(gf))
	cmd.AddCommand(newStatusCmd(gf))
	cmd.AddCommand(newApplyCmd(gf))

	return cmd
}

func countVerbose(cmd *cobra.Command) int {
	n, _ := cmd.Flags().GetCount("verbose")
	return n
}

// validateAndDisplayContext 验证工作目录并显示上下文信息
func validateAndDisplayContext(gf *GlobalFlags, out io.Writer) error {
	workdir := gf.Workdir

	// Step 1: 检查 workdir 是否存在
	if _, err := os.Stat(workdir); os.IsNotExist(err) {
		// 目录不存在，可能是首次 checkout，允许继续
		return nil
	}

	// Step 2: 检查 .svn 是否存在
	svnDir := filepath.Join(workdir, ".svn")
	isWC := false
	if info, err := os.Stat(svnDir); err == nil && info.IsDir() {
		isWC = true
	}

	if !isWC {
		// 不是工作副本
		if !gf.WorkdirExplicit {
			// 使用默认 -C 且无 .svn → 报错
			return fmt.Errorf("current directory is not an SVN working copy. Use -C to specify working directory")
		}
		// 显式指定 -C 且无 .svn → 允许（首次 checkout）
		return nil
	}

	// Step 3: .svn 存在，加载配置获取 URL
	cfg, err := config.Load(gf.ConfigFile)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	finalURL := cfg.URL
	if finalURL == "" {
		return fmt.Errorf("URL required: provide in config or --url flag")
	}

	// Step 4: 获取工作副本真实 URL
	wcURL, err := svn.GetWorkingCopyURL(context.Background(), svn.NewExecClient(), workdir)
	if err != nil {
		return fmt.Errorf("failed to get working copy URL: %w", err)
	}

	// Step 5: 比较 URL
	if wcURL != finalURL {
		return fmt.Errorf("URL mismatch. Working copy has %q, config specifies %q. Use \"svn switch\" to change the working copy URL, or update your config", wcURL, finalURL)
	}

	// Step 6: 存储解析后的 URL
	gf.ResolvedURL = finalURL

	return nil
}

// displayContext 显示上下文信息
func displayContext(out io.Writer, workdir, url, configFile string) {
	fmt.Fprintf(out, "Working directory: %s\n", workdir)
	fmt.Fprintf(out, "Repository URL:    %s\n", url)
	fmt.Fprintf(out, "Config file:       %s\n", configFile)
}

func Execute(version string) int {
	cmd := newRootCmd(version)
	cmd.SetOut(os.Stdout)
	cmd.SetErr(os.Stderr)

	if err := cmd.Execute(); err != nil {
		if ee, ok := err.(*exitError); ok {
			return ee.Code
		}
		return 2
	}
	return 0
}
