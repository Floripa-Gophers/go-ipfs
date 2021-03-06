package commands

import (
	"bytes"
	"fmt"
	cmds "github.com/ipfs/go-ipfs/commands"
	corerepo "github.com/ipfs/go-ipfs/core/corerepo"
	u "gx/ipfs/QmZNVWh8LLjAavuQ2JXuFmuYH3C11xo988vSgp7UQrTRj1/go-ipfs-util"
	"io"
)

var RepoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manipulate the IPFS repo.",
		ShortDescription: `
'ipfs repo' is a plumbing command used to manipulate the repo.
`,
	},

	Subcommands: map[string]*cmds.Command{
		"gc":   repoGcCmd,
		"stat": repoStatCmd,
	},
}

var repoGcCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Perform a garbage collection sweep on the repo.",
		ShortDescription: `
'ipfs repo gc' is a plumbing command that will sweep the local
set of stored objects and remove ones that are not pinned in
order to reclaim hard disk space.
`,
	},

	Options: []cmds.Option{
		cmds.BoolOption("quiet", "q", "Write minimal output."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		gcOutChan, err := corerepo.GarbageCollectAsync(n, req.Context())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)
			for k := range gcOutChan {
				outChan <- k
			}
		}()
	},
	Type: corerepo.KeyRemoved{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			outChan, ok := res.Output().(<-chan interface{})
			if !ok {
				return nil, u.ErrCast()
			}

			quiet, _, err := res.Request().Option("quiet").Bool()
			if err != nil {
				return nil, err
			}

			marshal := func(v interface{}) (io.Reader, error) {
				obj, ok := v.(*corerepo.KeyRemoved)
				if !ok {
					return nil, u.ErrCast()
				}

				buf := new(bytes.Buffer)
				if quiet {
					buf = bytes.NewBufferString(string(obj.Key) + "\n")
				} else {
					buf = bytes.NewBufferString(fmt.Sprintf("removed %s\n", obj.Key))
				}
				return buf, nil
			}

			return &cmds.ChannelMarshaler{
				Channel:   outChan,
				Marshaler: marshal,
				Res:       res,
			}, nil
		},
	},
}

var repoStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get stats for the currently used repo.",
		ShortDescription: `
'ipfs repo stat' is a plumbing command that will scan the local
set of stored objects and print repo statistics. It outputs to stdout:
NumObjects      int number of objects in the local repo
RepoSize        int size in bytes that the repo is currently taking
RepoPath        string the path to the repo being currently used	
`,
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		stat, err := corerepo.RepoStat(n, req.Context())
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		res.SetOutput(stat)
	},
	Options: []cmds.Option{
		cmds.BoolOption("human", "Output RepoSize in MiB."),
	},
	Type: corerepo.Stat{},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			stat, ok := res.Output().(*corerepo.Stat)
			if !ok {
				return nil, u.ErrCast()
			}

			human, _, err := res.Request().Option("human").Bool()
			if err != nil {
				return nil, err
			}

			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "NumObjects \t %d\n", stat.NumObjects)
			sizeInMiB := stat.RepoSize / (1024 * 1024)
			if human && sizeInMiB > 0 {
				fmt.Fprintf(buf, "RepoSize (MiB) \t %d\n", sizeInMiB)
			} else {
				fmt.Fprintf(buf, "RepoSize \t %d\n", stat.RepoSize)
			}
			fmt.Fprintf(buf, "RepoPath \t %s\n", stat.RepoPath)

			return buf, nil
		},
	},
}
