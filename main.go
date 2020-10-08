package main

import (
  "log"
  "net/http"
  "regexp"
  "strings"
  "sync"

  "github.com/gobuffalo/packr/v2"
  "github.com/judwhite/go-svc/svc"
  "github.com/spf13/viper"
  "go.uber.org/zap"
)

type program struct {
  wg   sync.WaitGroup
  quit chan struct{}
}

type configuration struct {
  HostURL      string
  Environments map[string]string
}

func (p *program) Init(env svc.Environment) error {
  log.Printf("is win service? %v\n", env.IsWindowsService())
  return nil
}

func (p *program) Start() error {
  // The Start method must not block, or Windows may assume your service failed
  // to start. Launch a Goroutine here to do something interesting/blocking.
  // init zap
  instance, err := zap.NewProduction()
  if err != nil {
    return err
  }
  zap.ReplaceGlobals(instance)

  if err != nil {
    zap.S().Errorw("Can not start", zap.Error(err))
  }
  // init config
  var config configuration
  viper.SetConfigFile("config.yaml")
  if err = viper.ReadInConfig(); err == nil {
    err = viper.Unmarshal(&config)
  }
  if err != nil {
    zap.S().Errorw("Could not init config", zap.Error(err))
    return err
  }
  box := packr.New("static", ".\\static")
  var validFile = regexp.MustCompile(^js\\app.*js$)
  for _, file := range box.List() {
    if validFile.MatchString(file) {
      f, _ := box.FindString(file)
      zap.S().Infow(file)
      for key, value := range config.Environments {
        f = strings.ReplaceAll(f, strings.ToUpper(key), value)
      }
      box.AddString(file, f)
    }
  }
  http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
    zap.S().Infow(r.URL.Path)
    if !box.Has(r.URL.Path) {
      index, _ := box.Find("index.html")
      w.Write(index)
      w.Header().Add("Content-Type", "text/html; charset=utf-8")
      return
    }
    http.FileServer(box).ServeHTTP(w, r)
  })
  go func() {
    if err := http.ListenAndServe(config.HostURL, nil); err != nil {
      zap.S().Errorw("Gateway: Failed to listen and serve", zap.Error(err))
      return
    }
  }()
  zap.S().Infof("Gateway is started on port %v", config.HostURL)
  zap.S().Info("*****RUNNING******")
  p.quit = make(chan struct{})

  p.wg.Add(1)
  go func() {
    log.Println("Starting...")
    <-p.quit
    log.Println("Quit signal received...")
    p.wg.Done()
  }()

  return nil
}

func (p *program) Stop() error {
  zap.S().Info("*****SHUTDOWN*****")
  log.Println("Stopping...")
  close(p.quit)
  p.wg.Wait()
  log.Println("Stopped.")
  return nil
}

func main() {
  prg := &program{}
  if err := svc.Run(prg); err != nil {
    log.Fatal(err)
  }
}