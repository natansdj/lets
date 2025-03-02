package loader

import (
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"strings"

	"github.com/natansdj/lets/drivers"
	"github.com/natansdj/lets/frameworks"
)

func Launching() {
	var printer = launcher{
		Width: 100,
	}
	printer.init()

	printer.hr()
	printer.printTitle("Let's Go Framework", "Framework is maintained by github.com/dhutapratama")

	if InfoIdentService != nil {
		printer.printStruct(*InfoIdentService)
	}

	if InfoIdentSource != nil {
		printer.printStruct(*InfoIdentSource)
	}

	if InfoNetwork != nil {
		printer.printHeading("Network")
		printer.printStruct(*InfoNetwork)
	}

	if InfoReplica != nil {
		printer.printStruct(*InfoReplica)
	}

	// Drivers
	if clients := drivers.MySQLConfig; clients != nil {
		for _, client := range clients {
			printer.printHeading("MySQL / MariaDB Clients")
			printer.printStruct(client)
		}
	}

	if drivers.RedisConfig != nil {
		printer.printHeading("Redis Client")
		printer.printStruct(drivers.RedisConfig)
	}
	if drivers.MongoDBConfig != nil {
		printer.printHeading("MongoDB Client")
		printer.printStruct(drivers.MongoDBConfig)
	}

	// Server / Client / Protovol
	if frameworks.HttpConfig != nil {
		printer.printHeading("HTTP Server")
		printer.printStruct(frameworks.HttpConfig)
	}

	if conf := frameworks.GrpcConfig; conf != nil {
		if conf.GetServer() != nil {
			printer.printHeading("gRPC Server")
			printer.printStruct(conf.GetServer())
		}
		if clients := conf.GetClients(); clients != nil {
			if len(clients) > 0 {
				printer.printHeading("gRPC Clients")
				for _, client := range clients {
					printer.printStruct(client)
				}
			}
		}
	}

	if frameworks.RabbitMQConfig != nil {
		if servers := frameworks.RabbitMQConfig.GetServers(); servers != nil {
			printer.printHeading("RabbitMQ Servers")

			for _, server := range servers {
				printer.printStruct(server)

				if publishers := server.GetPublishers(); publishers != nil {
					for _, publisher := range publishers {
						printer.printHeading("RabbitMQ Publisher")
						printer.printStruct(publisher)
					}
				}

				if consumers := server.GetConsumers(); consumers != nil {
     printer.printHeading("RabbitMQ Consumers")
     printer.printData("Count:", len(consumers))
     printer.hr()
     for _, consumer := range consumers {
						printer.printStruct(consumer)
					}
				}

			}
		}
	}
}

type launcher struct {
	Width int

	writer     io.Writer
	separator  string
	separator2 string
}

func (l *launcher) init() {
	l.separator = strings.Repeat("/", l.Width)
	l.separator += "\n"
	l.separator2 = "// " + strings.Repeat("-", l.Width-6) + " //"
	l.separator2 += "\n"

	l.writer = os.Stdout
}

func (l *launcher) printTitle(title string, maintener string) {
	var format = "// %-40s %53s //\n"

	fmt.Fprintf(l.writer, format, title, maintener)
	l.hr()
}

func (l *launcher) printHeading(title string) {
	lenTitle := len(title)
	spaceL := (l.Width - 6 - lenTitle) / 2
	spaceR := spaceL

	if (lenTitle % 2) == 1 {
		spaceR++
	}

	var format = "// %-" + fmt.Sprintf("%v", spaceL) + "s%s%-" + fmt.Sprintf("%v", spaceR) + "s //\n"

	fmt.Fprintf(l.writer, format, "", title, "")
	l.hr2()
}

func (l *launcher) printData(field string, value any) {
	if value == nil {
		return
	}

	switch reflect.TypeOf(value).Kind() {
	case reflect.String:
		var format = "// %-30s : %-61s //\n"
		fmt.Fprintf(l.writer, format, field, value)
	case reflect.Bool:
		var format = "// %-30s : %-61t //\n"
		fmt.Fprintf(l.writer, format, field, value)
	case reflect.Int:
		var format = "// %-30s : %-61d //\n"
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
	l.hr()
}

func (l *launcher) hr() {
	fmt.Fprintf(l.writer, "%s", l.separator)
}

func (l *launcher) hr2() {
	fmt.Fprintf(l.writer, "%s", l.separator2)
}
