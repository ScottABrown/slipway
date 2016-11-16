package main

import (
  "os"
  "fmt"
  "time"
  "strings"
  "strconv"
  "gopkg.in/urfave/cli.v1"
  "github.com/google/go-github/github"
)

var globalBuildVersion string

func CurrentVersion() string {
  if len(globalBuildVersion) == 0 {
    return "devel"
  } else {
    return "v"+globalBuildVersion
  }
}

func main() {
  year, _, _ := time.Now().Date()
  app := cli.NewApp()
  app.Name = "slipway"
  app.Version = CurrentVersion()
  app.Copyright = "© "+strconv.Itoa(year)+" Verizon Labs"
  app.Usage = "generate metadata and releases compatible with Nelson"
  app.EnableBashCompletion = true

  // switches for the cli
  var userDirectory string
  var userGithubHost string
  var userGithubTag string
  var userGithubRepoSlug string

  app.Commands = []cli.Command {
    ////////////////////////////// DEPLOYABLE //////////////////////////////////
    {
      Name:    "gen",
      Usage:   "generate deployable metdata for units",
      Flags: []cli.Flag {
        cli.StringFlag{
          Name:   "dir, d",
          Value:  "",
          Usage:  "location to output the YAML file",
          Destination: &userDirectory,
        },
      },
      Action:  func(c *cli.Context) error {
        if len(userDirectory) <= 0 {
          return cli.NewExitError("You must specify a '--dir' or '-d' flag with the destination directory for the deployable yml file.", 1)
        }
        return nil
      },
    },
    {
      Name:    "release",
      Usage:   "generate deployable metdata for units",
      Flags: []cli.Flag {
        cli.StringFlag {
          Name:   "endpoint, x",
          Value:  "",
          Usage:  "domain of the github api endpoint",
          EnvVar: "GITHUB_ADDR",
          Destination: &userGithubHost,
        },
        cli.StringFlag {
          Name:   "repo, r",
          Value:  "",
          Usage:  "the repository in question, e.g. verizon/knobs",
          EnvVar: "TRAVIS_REPO_SLUG",
          Destination: &userGithubRepoSlug,
        },
        cli.StringFlag {
          Name:   "tag, t",
          Value:  "",
          Usage:  "host of the github api endpoint",
          Destination: &userGithubTag,
        },
        cli.StringFlag {
          Name:   "dir, d",
          Value:  "",
          Usage:  "directory of .deployable.yml files to upload",
          Destination: &userDirectory,
        },
      },
      Action:  func(c *cli.Context) error {
        // deployables =

        if len(userGithubTag) <= 0  {
          return cli.NewExitError("You must specifiy a `--tag` or a `-t` to create releases.", 1)
        }
        if len(userGithubRepoSlug) <= 0  {
          return cli.NewExitError("You must specifiy a `--repo` or a `-r` to create releases.", 1)
        }

        splitarr := strings.Split(userGithubRepoSlug, "/")
        if len(splitarr) != 2 {
          return cli.NewExitError("The specified repository name was not of the format 'foo/bar'", 1)
        }

        owner := splitarr[0]
        reponame := splitarr[1]

        deployablePaths, direrr := findDeployableFilesInDir(userDirectory)

        if len(userDirectory) != 0 {
          // if you specified a dir, but it was not readable or it didnt exist
          if direrr != nil {
            return cli.NewExitError("Unable to read from "+userDirectory+"; check the location exists and is readable.", 1)
          }
          // if you specify a dir, and it was readable, but there were no deployable files
          if len(deployablePaths) <= 0 {
            return cli.NewExitError("Readable directory "+userDirectory+" contained no '.deployable.yml' files.", 1)
          }
        }

        credentials, err := loadGithubCredentials();
        if err == nil {
          gh := buildGithubClient(userGithubHost, credentials)

          name := GenerateRandomName()
          commitish := "master"
          isDraft := true

          // release structure
          r := github.RepositoryRelease {
            TagName: &userGithubTag,
            TargetCommitish: &commitish,
            Name: &name,
            Draft: &isDraft,
          }

          // create the release
          release, _, e := gh.Repositories.CreateRelease(owner, reponame, &r)

          fmt.Println("Created release "+strconv.Itoa(*release.ID)+" on "+owner+"/"+reponame)

          if e != nil {
            fmt.Println(e)
            return cli.NewExitError("Encountered an unexpected error whilst calling the specified Github endpint.", 1)
          }

          // upload the release assets
          for _, path := range deployablePaths {
            slices  := strings.Split(path, "/")
            name    := slices[len(slices)-1]
            file, _ := os.Open(path)

            fmt.Println("Uploading "+name+" as a release asset...")

            opt := &github.UploadOptions{ Name: name }
            gh.Repositories.UploadReleaseAsset(owner, reponame, *release.ID, opt, file)
          }

          fmt.Println("Promoting release from a draft to offical release...")

          // mutability ftw?
          isDraft = false
          r2 := github.RepositoryRelease {
            Draft: &isDraft,
          }

          _, _, xxx := gh.Repositories.EditRelease(owner, reponame, *release.ID, &r2)

          if xxx != nil {
            fmt.Println(xxx)
            return cli.NewExitError("Unable to promote this release to an offical release. Please ensure that the no other release references the same tag.", 1)
          }

        } else {
          return cli.NewExitError("Unable to load github credentials. Please ensure you have a valid properties file at $HOME/.github", 1)
        }

        return nil
      },
    },
  }

  // run it!
  app.Run(os.Args)
}