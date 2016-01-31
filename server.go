package main

import (
	"log"
	"os"
	"net"
	"io/ioutil"
	"time"
	"regexp"
	"strings"
	"strconv"
	"bufio"
)

type Server struct {
	conf Config
	lg log.Logger
	ch chan string
	chS chan string
}


// Sum metrics with prefix
func (s Server) sumMetricsWithPrefix() []string {
	for ;; time.Sleep(time.Duration(s.conf.SumInterval)*time.Second) {
		var working_list[] Metric
		chanSize := len(s.chS)
		for i := 0; i < chanSize; i++ {
			found := false
			metric := strings.Replace(<-s.chS, s.conf.SumPrefix, "", -1)
			split := regexp.MustCompile("\\s").Split(metric, 3)

			value, err := strconv.ParseFloat(split[1], 64)
			if err != nil {s.lg.Println("Can not parse value of a metric") ; continue}
			timestamp, err := strconv.ParseInt(split[2], 10, 64)
			if err != nil {s.lg.Println("Can not parse timestamp of a metric") ; continue}

			for i,_ := range working_list {
				if working_list[i].name == split[0] {
					working_list[i].amount++
					working_list[i].value += value
					working_list[i].timestamp += timestamp
					found = true
					break
				}
			}
			if !found {
				working_list = append(working_list, Metric{split[0], 1, value, timestamp})
			}
		}
		for _,val := range working_list {
			s.ch <- val.name + " " +
				strconv.FormatFloat(val.value, 'f', 2, 32) + " " + strconv.FormatInt(val.timestamp/val.amount, 10)
		}
	}
}

// Function checks and removed bad data and sorts it by SUM prefix
func (s Server)cleanAndSortIncomingData(metrics []string) {
	for _,metric := range metrics {
		if validateMetric(metric) {
			if strings.HasPrefix(metric, s.conf.SumPrefix) {
				if len(s.chS) < s.conf.MaxMetrics*s.conf.SumInterval{
					s.chS <- metric
				}
			} else {
				if len(s.ch) < s.conf.MaxMetrics*s.conf.ClientSendInterval {
					s.ch <- metric
				}
			}
		}else {
			s.lg.Println("Removing bad metric \"" + metric + "\" from the list")
		}
	}
	metrics = nil
}

// Handles incoming requests.
func (s Server)handleRequest(conn net.Conn) {
	connbuf := bufio.NewReader(conn)
	var results_list []string
	for i:=0; i< s.conf.MaxMetrics; i++ {
		metric, err := connbuf.ReadString('\n')
		results_list = append(results_list, strings.Replace(metric, "\n", "", -1))
		if err!= nil {
			break
		}
	}
	conn.Close()
	// We have to cut last element cause it is always empty: ""
	s.cleanAndSortIncomingData(results_list[:len(results_list)-1])
	results_list = nil
}

// Reading metrics from files in folder. This is a second way how to send metrics, except direct connection
func (s Server)handleDirMetrics() []string {
	for ;; time.Sleep(time.Duration(s.conf.ClientSendInterval)*time.Second) {
		var results_list []string
		files, err := ioutil.ReadDir(s.conf.MetricDir)
		if err != nil {
			panic(err.Error())
			return results_list
		}
		for _, f := range files {
			s.cleanAndSortIncomingData(readMetricsFromFile(s.conf.MetricDir+"/"+f.Name()))
		}

	}
}

func (s Server)runServer() {
	// Listen for incoming connections.
	l, err := net.Listen("tcp", s.conf.LocalBind)
	if err != nil {
		s.lg.Println("Failed to Run server:", err.Error())
		os.Exit(1)
	} else {
		s.lg.Println("Server is running")
	}
	// Close the listener when the application closes.
	defer l.Close()

	// Run goroutine for reading metrics from metricDir
	go s.handleDirMetrics()
	// Run goroutine for sum metrics with prefix
	go s.sumMetricsWithPrefix()

	for {
		// Listen for an incoming connection.
		conn, err := l.Accept()
		if err != nil {
			s.lg.Println("Error accepting: ", err.Error())
			os.Exit(1)
		}
		// Handle connections in a new goroutine.
		go s.handleRequest(conn)
	}
}