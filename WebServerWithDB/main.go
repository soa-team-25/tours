package main

import (
	"context"
	"database-example/handler"
	"database-example/model"
	"database-example/proto/tour"
	"database-example/repo"
	"database-example/service"
	"log"
	stdnet "net"
	"net/http"
	"time"

	//"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const serviceName = "tours"

// // prometius da se pokrece?
// var (
// 	requestsTotal = prometheus.NewCounterVec(
// 		prometheus.CounterOpts{
// 			Name: "http_requests_total",
// 			Help: "Total number of HTTP requests",
// 		},
// 		[]string{"method", "endpoint"},
// 	)
// 	requestDuration = prometheus.NewHistogramVec(
// 		prometheus.HistogramOpts{
// 			Name: "http_request_duration_seconds",
// 			Help: "Histogram of response latency (seconds) of HTTP requests.",
// 		},
// 		[]string{"method", "endpoint"},
// 	)
// )

// func init() {
// 	prometheus.MustRegister(requestsTotal, requestDuration)
// }

// func instrumentHandler(next http.HandlerFunc) http.HandlerFunc {
// 	return func(w http.ResponseWriter, r *http.Request) {
// 		endpoint := r.URL.Path
// 		method := r.Method

// 		timer := prometheus.NewTimer(requestDuration.WithLabelValues(method, endpoint))
// 		defer timer.ObserveDuration()

// 		requestsTotal.WithLabelValues(method, endpoint).Inc()

// 		next.ServeHTTP(w, r)
// 	}
// }

var (
	cpuUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "host_cpu_usage_percent",
		Help: "Current CPU usage percentage",
	})
	memUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "host_mem_usage_percent",
		Help: "Current memory usage percentage",
	})
	diskUsage = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "host_disk_usage_percent",
		Help: "Current disk usage percentage",
	})
	netSent = promauto.NewCounter(prometheus.CounterOpts{
		Name: "host_network_bytes_sent_total",
		Help: "Total bytes sent over the network",
	})
	netRecv = promauto.NewCounter(prometheus.CounterOpts{
		Name: "host_network_bytes_received_total",
		Help: "Total bytes received over the network",
	})
)

func collectMetrics() {
	for {
		// Collect CPU usage
		cpuPercent, err := cpu.Percent(time.Second, false)
		if err == nil && len(cpuPercent) > 0 {
			cpuUsage.Set(cpuPercent[0])
		}

		// Collect memory usage
		virtualMem, err := mem.VirtualMemory()
		if err == nil {
			memUsage.Set(virtualMem.UsedPercent)
		}

		// Collect disk usage
		diskInfo, err := disk.Usage("/")
		if err == nil {
			diskUsage.Set(diskInfo.UsedPercent)
		}

		// Collect network usage
		netIO, err := net.IOCounters(false)
		if err == nil && len(netIO) > 0 {
			netSent.Add(float64(netIO[0].BytesSent))
			netRecv.Add(float64(netIO[0].BytesRecv))
		}

		time.Sleep(10 * time.Second) // Adjust the collection interval as needed
	}
}

func initDB() *gorm.DB {
	dsn := "user=postgres password=super dbname=explorer host=database port=5432 sslmode=disable search_path=tours"
	database, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		print(err)
		return nil
	}

	database.AutoMigrate(&model.Tour{}, &model.TourPoint{}, &model.TourReview{}, &model.TourObject{}, &model.PublicTourPoint{})
	return database
}

func startTourServer(handler *handler.TourHandler, tourObjectHandler *handler.TourObjectHandler, tourPointHandler *handler.TourPointHandler,
	tourPointRequestHandler *handler.TourPointRequestHandler, publicTourPointHandler *handler.PublicTourPointHandler) {

	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/tours/{id}", handler.Get).Methods("GET")
	router.HandleFunc("/tours/create", handler.Create).Methods("POST")
	router.HandleFunc("/tours/getByAuthor/{userId}", handler.GetByUserId).Methods("GET")
	router.HandleFunc("/tours/setCaracteristics/{id}", handler.AddCharacteristics).Methods("PUT")
	router.HandleFunc("/tours/publish/{tourId}", handler.Publish).Methods("PUT")
	router.HandleFunc("/tours/archive/{tourId}", handler.Archive).Methods("PUT")
	router.HandleFunc("/tours/delete/{tourId}", handler.Delete).Methods("DELETE")

	router.HandleFunc("/tourPoint/create", tourPointHandler.Create).Methods("POST")
	router.HandleFunc("/tourPoint/getAll", tourPointHandler.GetAll).Methods("GET")
	router.HandleFunc("/tourPoint/getById/{id}", tourPointHandler.GetById).Methods("GET")

	router.HandleFunc("/tourObjects/{id}", tourObjectHandler.Get).Methods("GET")
	router.HandleFunc("/tourObjects/create", tourObjectHandler.Create).Methods("POST")

	router.HandleFunc("/tourPointRequest/create", tourPointRequestHandler.Create).Methods("POST")
	router.HandleFunc("/tourPointRequest/accept/{tourPointRequestId}", tourPointRequestHandler.AcceptRequest).Methods("PUT")
	router.HandleFunc("/tourPointRequest/decline/{tourPointRequestId}", tourPointRequestHandler.DeclineRequest).Methods("PUT")

	router.HandleFunc("/publicTourPoint/setPublicTourPoint/{tourPointId}", publicTourPointHandler.CreateFromTourPointId).Methods("GET")

	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static")))
	log.Println("Server starting on port 3200")

	log.Fatal(http.ListenAndServe(":3200", router))
}

func main() {
	database := initDB()
	if database == nil {
		log.Fatal("Failed to connect to the database")
		return
	}

	tourRepo := &repo.TourRepository{DatabaseConnection: database}
	tourService := &service.TourService{TourRepo: tourRepo}

	// router := mux.NewRouter().StrictSlash(true)
	// router.Handle("/metrics", promhttp.Handler())
	//------------------------------------------------------
	// http.Handle("/metrics", promhttp.Handler())
	// http.ListenAndServe(":3200", nil) //tours:3200?
	// go collectMetrics()

	// lis, err := stdnet.Listen("tcp", "tours:3300")
	// if err != nil {
	// 	log.Fatalf("failed to listen: %v", err)
	// }
	//------------------------------------------
	// Start HTTP server for metrics on port 3200
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Println("Starting METRICS server on port 3200")
		log.Fatal(http.ListenAndServe(":3200", nil))
	}()

	// Start metrics collection in a goroutine
	go collectMetrics()

	// Start gRPC server on port 3300
	lis, err := stdnet.Listen("tcp", ":3300")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	tour.RegisterTourServer(grpcServer, &Server{TourService: tourService})

	reflection.Register(grpcServer)
	log.Println("------------------------Starting gRPC server starting on port 3300!!!!!--------------------")
	grpcServer.Serve(lis)
}

type Server struct {
	tour.UnimplementedTourServer
	TourService *service.TourService
}

func (s *Server) Create(ctx context.Context, request *tour.TourDto) (*tour.TourDto, error) {
	// Map TourCharacteristicDto to model.TourCharacteristic
	println("Usao je ovde CREATE TOURS")
	var characteristics model.TourCharacteristicsSlice
	for _, c := range request.TourCharacteristics {
		characteristics = append(characteristics, model.TourCharacteristic{
			Distance:      c.Distance,
			Duration:      c.Duration,
			TransportType: c.TransportType,
		})
	}

	t := model.Tour{
		Name:                request.Name,
		Description:         request.Description,
		UserID:              int(request.UserId),
		DifficultyLevel:     request.DifficultyLevel,
		Tags:                request.Tags,
		Status:              request.Status,
		Price:               int(request.Price),
		PublishedDateTime:   request.PublishedDateTime.AsTime(),
		ArchivedDateTime:    request.ArchivedDateTime.AsTime(),
		TourCharacteristics: characteristics,
	}

	err := s.TourService.Create(&t)
	if err != nil {
		return nil, err
	}

	response := &tour.TourDto{
		Id:              int64(t.ID),
		Name:            t.Name,
		Description:     t.Description,
		DifficultyLevel: t.DifficultyLevel,
		Tags:            t.Tags,
		Status:          t.Status,
		Price:           int32(t.Price),
		UserId:          int64(t.UserID),
	}

	return response, nil
}

func (s *Server) GetByUserId(ctx context.Context, request *tour.PageRequestTour) (*tour.TourListResponse, error) {
	// Poziv metode FindByUserId iz TourService-a
	print("Usao je tours!")
	tours, err := s.TourService.FindByUserId(int(request.UserId))
	if err != nil {
		return nil, err
	}

	// Mapiranje tours u listu TourDto objekata
	var tourDtos []*tour.TourDto
	for _, t := range tours {
		tourDto := &tour.TourDto{
			Id:                int64(t.ID),
			Name:              t.Name,
			PublishedDateTime: timestamppb.New(t.PublishedDateTime),
			ArchivedDateTime:  timestamppb.New(t.ArchivedDateTime),
			Description:       t.Description,
			DifficultyLevel:   t.DifficultyLevel,
			Tags:              t.Tags,
			Price:             int32(t.Price),
			Status:            t.Status,
			UserId:            int64(t.UserID),
		}
		tourDtos = append(tourDtos, tourDto)
	}

	// Kreiranje TourListResponse objekta
	response := &tour.TourListResponse{
		Results:    tourDtos,
		TotalCount: int32(len(tours)),
	}

	return response, nil
}

func (s *Server) Publish(ctx context.Context, request *tour.TourPublishRequest) (*tour.TourDto, error) {

	publishedTour, err := s.TourService.PublishTourNEW(int(request.TourId))
	if err != nil {
		return nil, err
	}

	// Mapiranje model.Tour na tour.TourDto
	response := &tour.TourDto{
		Id:                int64(publishedTour.ID),
		Name:              publishedTour.Name,
		Description:       publishedTour.Description,
		DifficultyLevel:   publishedTour.DifficultyLevel,
		Tags:              publishedTour.Tags,
		Price:             int32(publishedTour.Price),
		Status:            publishedTour.Status,
		UserId:            int64(publishedTour.UserID),
		PublishedDateTime: timestamppb.New(publishedTour.PublishedDateTime),
		ArchivedDateTime:  timestamppb.New(publishedTour.ArchivedDateTime),
	}

	return response, nil
}

func (s *Server) Archive(ctx context.Context, request *tour.TourPublishRequest) (*tour.TourDto, error) {

	archiveTour, err := s.TourService.ArchiveTourNEW(int(request.TourId))
	if err != nil {
		return nil, err
	}

	// Mapiranje model.Tour na tour.TourDto
	response := &tour.TourDto{
		Id:                int64(archiveTour.ID),
		Name:              archiveTour.Name,
		Description:       archiveTour.Description,
		DifficultyLevel:   archiveTour.DifficultyLevel,
		Tags:              archiveTour.Tags,
		Price:             int32(archiveTour.Price),
		Status:            archiveTour.Status,
		UserId:            int64(archiveTour.UserID),
		PublishedDateTime: timestamppb.New(archiveTour.PublishedDateTime),
		ArchivedDateTime:  timestamppb.New(archiveTour.ArchivedDateTime),
	}

	return response, nil
}

func (s *Server) Delete(ctx context.Context, request *tour.TourIdRequest) (*tour.TourDto, error) {
	// Pozovi DeleteTourNEW metodu iz servisa
	deletedTour, err := s.TourService.DeleteTourNEW(int(request.Id))
	if err != nil {
		return nil, err
	}

	// Mapiranje model.Tour na tour.TourDto
	response := &tour.TourDto{
		Id:                int64(deletedTour.ID),
		Name:              deletedTour.Name,
		Description:       deletedTour.Description,
		DifficultyLevel:   deletedTour.DifficultyLevel,
		Tags:              deletedTour.Tags,
		Price:             int32(deletedTour.Price),
		Status:            deletedTour.Status,
		UserId:            int64(deletedTour.UserID),
		PublishedDateTime: timestamppb.New(deletedTour.PublishedDateTime),
		ArchivedDateTime:  timestamppb.New(deletedTour.ArchivedDateTime),
	}

	return response, nil
}
