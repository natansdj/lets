package loader

import (
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"

	"github.com/natansdj/lets"
	"github.com/natansdj/lets/drivers"
	"github.com/natansdj/lets/frameworks"
)

func Launching() {
	// Check if startup banner is enabled
	if SystemInfoConfig != nil && !SystemInfoConfig.EnableStartupBanner() {
		lets.LogI("LETS Framework initialized (startup banner disabled)")
		return
	}

	var printer = launcher{
		Width: 100,
	}
	printer.init()

	printer.hr()
	printer.printTitle("Let's Go Framework", "v2.0")

	if InfoIdentService != nil {
		printer.printHeading("Service Information")
		printer.printStruct(*InfoIdentService)
	}

	if InfoIdentSource != nil {
		printer.printHeading("Source Information")
		printer.printStruct(*InfoIdentSource)
	}

	// Only show network info if enabled
	if SystemInfoConfig == nil || SystemInfoConfig.EnableNetworkInfo() {
		if InfoNetwork != nil && (len(InfoNetwork.IPV4) > 0 || len(InfoNetwork.IPV6) > 0) {
			printer.printHeading("Network Configuration")
			printer.printStruct(*InfoNetwork)
		}
	}

	// Only show replica info if enabled
	if SystemInfoConfig == nil || SystemInfoConfig.EnableReplicaInfo() {
		if InfoReplica != nil && (len(InfoReplica.IPV4) > 0 || len(InfoReplica.IPV6) > 0) {
			printer.printHeading("Replica Configuration")
			printer.printStruct(*InfoReplica)
		}
	}

	// Drivers section
	if clients := drivers.MySQLConfig; len(clients) > 0 {
		printer.printSection("Database Drivers")
		printer.printHeading("MySQL / MariaDB Clients")
		for i, client := range clients {
			if len(clients) > 1 {
				printer.printSubHeading(fmt.Sprintf("Client #%d", i+1))
			}
			printer.printStruct(client)
		}
	}

	if drivers.RedisConfig != nil {
		if len(drivers.MySQLConfig) == 0 {
			printer.printSection("Database Drivers")
		}
		printer.printHeading("Redis Client")
		printer.printStruct(drivers.RedisConfig)
	}

	if drivers.MongoDBConfig != nil {
		if len(drivers.MySQLConfig) == 0 && drivers.RedisConfig == nil {
			printer.printSection("Database Drivers")
		}
		printer.printHeading("MongoDB Client")
		printer.printStruct(drivers.MongoDBConfig)
	}

	// Server / Client / Protocol section
	if frameworks.HttpConfig != nil {
		printer.printSection("Framework Services")
		printer.printHeading("HTTP Server")
		printer.printStruct(frameworks.HttpConfig)
	}

	if conf := frameworks.GrpcConfig; conf != nil {
		if frameworks.HttpConfig == nil {
			printer.printSection("Framework Services")
		}
		if conf.GetServer() != nil {
			printer.printHeading("gRPC Server")
			printer.printStruct(conf.GetServer())
		}
		if clients := conf.GetClients(); len(clients) > 0 {
			printer.printHeading("gRPC Clients")
			for i, client := range clients {
				if len(clients) > 1 {
					printer.printSubHeading(fmt.Sprintf("Client #%d", i+1))
				}
				printer.printStruct(client)
			}
		}
	}

	if frameworks.RabbitMQConfig != nil {
		if servers := frameworks.RabbitMQConfig.GetServers(); len(servers) > 0 {
			if frameworks.HttpConfig == nil && frameworks.GrpcConfig == nil {
				printer.printSection("Framework Services")
			}
			printer.printHeading("RabbitMQ Servers")

			for i, server := range servers {
				if len(servers) > 1 {
					printer.printSubHeading(fmt.Sprintf("Server #%d", i+1))
				}
				printer.printStruct(server)

				if publishers := server.GetPublishers(); len(publishers) > 0 {
					printer.printHeading("RabbitMQ Publishers")
					for j, publisher := range publishers {
						if len(publishers) > 1 {
							printer.printSubHeading(fmt.Sprintf("Publisher #%d", j+1))
						}
						printer.printStruct(publisher)
					}
				}

				if consumers := server.GetConsumers(); len(consumers) > 0 {
					printer.printHeading("RabbitMQ Consumers")
					printer.printData("Consumer Count", len(consumers))
					printer.hr2()
					for j, consumer := range consumers {
						if len(consumers) > 1 {
							printer.printSubHeading(fmt.Sprintf("Consumer #%d", j+1))
						}
						printer.printStruct(consumer)
					}
				}
			}
		}
	}

	printer.hr()
	printer.printFooter("Ready to serve")
}

type launcher struct {
	Width int

	writer     io.Writer
	separator  string
	separator2 string
	separator3 string
}

func (l *launcher) init() {
	l.separator = strings.Repeat("═", l.Width)
	l.separator += "\n"
	l.separator2 = "╠═ " + strings.Repeat("─", l.Width-6) + " ═╣"
	l.separator2 += "\n"
	l.separator3 = "├─ " + strings.Repeat("·", l.Width-6) + " ─┤"
	l.separator3 += "\n"

	l.writer = os.Stdout
}

func (l *launcher) printTitle(title string, version string) {
	titleStr := title
	if version != "" {
		titleStr = fmt.Sprintf("%s %s", title, version)
	}

	lenTitle := len(titleStr)
	spaceL := (l.Width - lenTitle) / 2
	spaceR := spaceL

	if (lenTitle % 2) == 1 {
		spaceR++
	}

	var format = "║ %-" + fmt.Sprintf("%v", spaceL-1) + "s%s%-" + fmt.Sprintf("%v", spaceR-1) + "s ║\n"

	fmt.Fprintf(l.writer, format, "", titleStr, "")
	l.hr()
}

func (l *launcher) printSection(title string) {
	fmt.Fprintf(l.writer, "\n")
	l.printHeading(title)
}

func (l *launcher) printHeading(title string) {
	lenTitle := len(title)
	spaceL := (l.Width - 6 - lenTitle) / 2
	spaceR := spaceL

	if (lenTitle % 2) == 1 {
		spaceR++
	}

	var format = "║ %-" + fmt.Sprintf("%v", spaceL) + "s%s%-" + fmt.Sprintf("%v", spaceR) + "s ║\n"

	fmt.Fprintf(l.writer, format, "", title, "")
	l.hr2()
}

func (l *launcher) printSubHeading(title string) {
	var format = "║ ▸ %-" + fmt.Sprintf("%v", l.Width-6) + "s ║\n"
	fmt.Fprintf(l.writer, format, title)
	l.hr3()
}

func (l *launcher) printFooter(message string) {
	lenMsg := len(message)
	spaceL := (l.Width - lenMsg) / 2
	spaceR := spaceL

	if (lenMsg % 2) == 1 {
		spaceR++
	}

	var format = "║ %-" + fmt.Sprintf("%v", spaceL-1) + "s%s%-" + fmt.Sprintf("%v", spaceR-1) + "s ║\n"

	fmt.Fprintf(l.writer, format, "", message, "")
}

func (l *launcher) printData(field string, value any) {
	if value == nil {
		return
	}

	fieldLen := 35
	valueLen := l.Width - fieldLen - 6 // 6 for borders and spacing

	switch reflect.TypeOf(value).Kind() {
	case reflect.String:
		strVal := value.(string)
		if len(strVal) > valueLen {
			strVal = strVal[:valueLen-3] + "..."
		}
		var format = "║ %-" + fmt.Sprintf("%d", fieldLen) + "s : %-" + fmt.Sprintf("%d", valueLen) + "s ║\n"
		fmt.Fprintf(l.writer, format, field, strVal)
	case reflect.Bool:
		status := "✗"
		if value.(bool) {
			status = "✓"
		}
		var format = "║ %-" + fmt.Sprintf("%d", fieldLen) + "s : %-" + fmt.Sprintf("%d", valueLen) + "s ║\n"
		fmt.Fprintf(l.writer, format, field, status)
	case reflect.Int:
		var format = "║ %-" + fmt.Sprintf("%d", fieldLen) + "s : %-" + fmt.Sprintf("%d", valueLen) + "d ║\n"
		fmt.Fprintf(l.writer, format, field, value)
	}
}

func (l *launcher) printStruct(info any) {
	rt := reflect.TypeOf(info)

	if rt.Kind() == reflect.Pointer {
		data := reflect.ValueOf(info).Elem().Interface()
		l.printStruct(data)
		return
	}

	if rt.Kind() != reflect.Struct {
		panic("bad type" + rt.Kind().String())
	}

	v := reflect.ValueOf(info)
	for i := 0; i < v.NumField(); i++ {
		f := rt.Field(i)

		name := f.Name
		if f.Tag.Get("desc") != "" {
			name = strings.Split(f.Tag.Get("desc"), ",")[0]
		}

		if f.Type.Kind() == reflect.Slice {

			if f.Type.String() == "[]string" {
				vals := v.Field(i).Interface().([]string)
				for _, val := range vals {
					l.printData(name, val)
				}
			} else if f.Type.String() == "[]net.IP" {
				vals := v.Field(i).Interface().([]net.IP)
				for _, val := range vals {
					l.printData(name, val.String())
				}
			} else {
				// lets.LogD("Unknown Type: %s", f.Type.String())
			}
		} else {
			if f.Tag.Get("hidden") != "" {
				return
			}
			if v.Field(i).Interface() != "" {
				l.printData(name, v.Field(i).Interface())
			}
		}

		// switch reflect.TypeOf(i).Kind() {
		// case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice:
		// 	return reflect.ValueOf(i).IsNil()
		// }
	}
	l.hr3()
}

func (l *launcher) hr() {
	fmt.Fprintf(l.writer, "%s", l.separator)
}

func (l *launcher) hr2() {
	fmt.Fprintf(l.writer, "%s", l.separator2)
}

func (l *launcher) hr3() {
	fmt.Fprintf(l.writer, "%s", l.separator3)
}
