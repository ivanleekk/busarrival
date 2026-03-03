export interface BusStop {
  BusStopCode: string;
  RoadName: string;
  Description: string;
  Latitude: number;
  Longitude: number;
}

export interface GroupedInterval {
  hour: number;
  averageMinutes: number;
  variance: number;
  standardDev: number;
  count: number;
}

export interface IntervalAnalysisResponse {
  stopCode: string;
  busNumber: string;
  hourlyData: GroupedInterval[];
}

export interface NextBus {
  OriginCode: string;
  DestinationCode: string;
  EstimatedArrival: string;
  Latitude: string;
  Longitude: string;
  VisitNumber: string;
  Load: string;
  Feature: string;
  Type: string;
}

export interface BusArrivalService {
  ServiceNo: string;
  Operator: string;
  NextBus: NextBus;
  NextBus2: NextBus;
  NextBus3: NextBus;
}

export interface BusArrivalResponse {
  odata_metadata: string;
  BusStopCode: string;
  Services: BusArrivalService[];
}
