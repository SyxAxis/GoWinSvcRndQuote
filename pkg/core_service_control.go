package pkg

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/debug"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"
)

type myservice struct{}

var windowsEventlog debug.Log

/*

	Standard Service Control Code Below Here

*/

func ServiceControl(operation string) {

	inService, err := svc.IsWindowsService()
	if err != nil {
		log.Fatalf("failed to determine if we are running in service: %v", err)
	}
	if inService {
		runService(svcSystemName, false)
		return
	}

	if operation == "" {
		Usage("ServiceControl : no command specified")
	}

	cmd := strings.ToLower(operation)
	switch cmd {
	case "debug":
		runService(svcSystemName, true)
		return
	case "install":
		// SERVICE NAME(system) and DISPLAY_NAME(visible)
		err = installService(svcSystemName, svcDisplayName)
	case "remove":
		err = removeService(svcSystemName)
	case "start":
		err = startService(svcSystemName)
	case "stop":
		err = controlService(svcSystemName, svc.Stop, svc.Stopped)
	case "pause":
		err = controlService(svcSystemName, svc.Pause, svc.Paused)
	case "continue":
		err = controlService(svcSystemName, svc.Continue, svc.Running)
	default:
		Usage(fmt.Sprintf("invalid command %s", cmd))
	}
	if err != nil {
		log.Fatalf("failed to %s %s: %v", cmd, svcSystemName, err)
	}
	// return
}

func Usage(errmsg string) {
	fmt.Fprintf(os.Stderr,
		"%s\n\n"+
			"usage: %s <command>\n"+
			"       where <command> is one of\n"+
			"       install, remove, debug, start, stop, pause or resume.\n",
		errmsg, os.Args[0])
	os.Exit(2)
}

/*
	INSTALLER / REMOVER
*/

func installService(sysname, displayname string) error {
	exepath, err := exePath()
	if err != nil {
		return err
	}
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(sysname)
	if err == nil {
		s.Close()
		return fmt.Errorf("service %s already exists", sysname)
	}
	s, err = m.CreateService(sysname, exepath, mgr.Config{DisplayName: displayname, Description: svcDescription}, "is", "auto-started")
	if err != nil {
		return err
	}
	defer s.Close()
	err = eventlog.InstallAsEventCreate(sysname, eventlog.Error|eventlog.Warning|eventlog.Info)
	if err != nil {
		s.Delete()
		return fmt.Errorf("SetupEventLogSource() failed: %s", err)
	}
	return nil
}

func removeService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("service %s is not installed", name)
	}
	defer s.Close()
	err = s.Delete()
	if err != nil {
		return err
	}
	err = eventlog.Remove(name)
	if err != nil {
		return fmt.Errorf("RemoveEventLogSource() failed: %s", err)
	}
	return nil
}

func exePath() (string, error) {
	prog := os.Args[0]
	p, err := filepath.Abs(prog)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(p)
	if err == nil {
		if !fi.Mode().IsDir() {
			return p, nil
		}
		err = fmt.Errorf("%s is directory", p)
	}
	if filepath.Ext(p) == "" {
		p += ".exe"
		fi, err := os.Stat(p)
		if err == nil {
			if !fi.Mode().IsDir() {
				return p, nil
			} else {
				err = fmt.Errorf("%s is directory", p)
				return p, err
			}
		}
	}
	return "", err
}

/*
	SERVICE CONTROL
*/

func startService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	err = s.Start("is", "manual-started")
	if err != nil {
		return fmt.Errorf("could not start service: %v", err)
	}
	return nil
}

func controlService(name string, c svc.Cmd, to svc.State) error {
	m, err := mgr.Connect()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("could not access service: %v", err)
	}
	defer s.Close()
	status, err := s.Control(c)
	if err != nil {
		return fmt.Errorf("could not send control=%d: %v", c, err)
	}
	timeout := time.Now().Add(10 * time.Second)
	for status.State != to {
		if timeout.Before(time.Now()) {
			return fmt.Errorf("timeout waiting for service to go to state=%d", to)
		}
		time.Sleep(300 * time.Millisecond)
		status, err = s.Query()
		if err != nil {
			return fmt.Errorf("could not retrieve service status: %v", err)
		}
	}
	return nil
}

func runService(name string, isDebug bool) {
	var err error
	if isDebug {
		windowsEventlog = debug.New(name)
	} else {
		windowsEventlog, err = eventlog.Open(name)
		if err != nil {
			return
		}
	}
	defer windowsEventlog.Close()

	windowsEventlog.Info(1, fmt.Sprintf("starting %s service", name))
	run := svc.Run
	if isDebug {
		run = debug.Run
	}
	err = run(name, &myservice{})
	if err != nil {
		windowsEventlog.Error(1, fmt.Sprintf("%s service failed: %v", name, err))
		return
	}
	windowsEventlog.Info(1, fmt.Sprintf("%s service stopped", name))
}

/*
	SERVICE RUNTIME
*/

func (m *myservice) Execute(args []string, svcChgRqstCh <-chan svc.ChangeRequest, svcStatusCh chan<- svc.Status) (ssec bool, errno uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown | svc.AcceptPauseAndContinue
	// we're keeping the Windows Service Manager updated on what we're up to in here
	// started so tell it the start up is pending
	svcStatusCh <- svc.Status{State: svc.StartPending}
	// now we have it running and we'll accept new svc commands
	svcStatusCh <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	windowsEventlog.Info(1, "Service running...")

	/*
		CUSTOM CODE CALL STARTS HERE
	*/
	go func() {
		InitCustomCode()
	}()

	// this loop is watching for service state change requests such as start, stop, status, et
loop:

	for c := range svcChgRqstCh {
		switch c.Cmd {
		case svc.Interrogate:
			svcStatusCh <- c.CurrentStatus
			// Testing deadlock from https://code.google.com/p/winsvc/issues/detail?id=4
			time.Sleep(100 * time.Millisecond)
			svcStatusCh <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			// golang.org/x/sys/windows/svc.TestExample is verifying this output.
			testOutput := strings.Join(args, "-")
			testOutput += fmt.Sprintf("-%d", c.Context)
			windowsEventlog.Info(1, testOutput)
			break loop
		case svc.Pause:
			svcStatusCh <- svc.Status{State: svc.Paused, Accepts: cmdsAccepted}
		case svc.Continue:
			svcStatusCh <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
		default:
			windowsEventlog.Error(1, fmt.Sprintf("unexpected control request #%d", c))
		}

	}

	svcStatusCh <- svc.Status{State: svc.StopPending}
	return

}
