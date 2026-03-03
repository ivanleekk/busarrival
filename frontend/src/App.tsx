import { useState, useEffect, useMemo } from 'react';
import axios from 'axios';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import type { BusStop, BusArrivalResponse, IntervalAnalysisResponse, BusArrivalService } from './types';
import './App.css';

const API_BASE = 'http://localhost:8080';

function App() {
  const [stops, setStops] = useState<BusStop[]>([]);
  const [selectedStop, setSelectedStop] = useState<string>('');

  const [services, setServices] = useState<string[]>([]);
  const [selectedService, setSelectedService] = useState<string>('');

  const [liveArrivals, setLiveArrivals] = useState<BusArrivalService | null>(null);
  const [analysisData, setAnalysisData] = useState<IntervalAnalysisResponse | null>(null);

  const [loadingStops, setLoadingStops] = useState(false);

  useEffect(() => {
    // Fetch all bus stops
    setLoadingStops(true);
    axios.get(`${API_BASE}/busstops`)
      .then(res => {
        // Assuming response structure { odata_metadata: string, value: BusStop[] } or { BusStops: BusStop[] }
        const data = res.data.BusStops || res.data.value || [];
        setStops(data);
        if (data.length > 0) {
          setSelectedStop(data[0].BusStopCode);
        }
      })
      .catch(err => console.error(err))
      .finally(() => setLoadingStops(false));
  }, []);

  // When a stop is selected, fetch live arrivals to see what services are available at this stop
  useEffect(() => {
    if (!selectedStop) return;

    axios.get<BusArrivalResponse>(`${API_BASE}/api/arrivals/${selectedStop}`)
      .then(res => {
        const arrivalData = res.data;
        const availableServices = arrivalData.Services.map(s => s.ServiceNo);
        setServices(availableServices);

        if (availableServices.length > 0) {
          if (!availableServices.includes(selectedService)) {
            setSelectedService(availableServices[0]);
          }
        } else {
          setSelectedService('');
        }

        if (selectedService) {
           const srv = arrivalData.Services.find(s => s.ServiceNo === selectedService);
           setLiveArrivals(srv || null);
        }
      })
      .catch(err => console.error(err));
  }, [selectedStop, selectedService]);

  // When stop or service changes, fetch historical analysis
  useEffect(() => {
    if (!selectedStop || !selectedService) {
      setAnalysisData(null);
      return;
    }

    axios.get<IntervalAnalysisResponse>(`${API_BASE}/api/analysis/intervals`, {
      params: { stopCode: selectedStop, busNumber: selectedService }
    })
      .then(res => {
        setAnalysisData(res.data);
      })
      .catch(err => {
        console.error(err);
        setAnalysisData(null);
      });
  }, [selectedStop, selectedService]);

  const chartData = useMemo(() => {
    if (!analysisData || !analysisData.hourlyData) return [];
    return analysisData.hourlyData.map(d => ({
      hour: `${d.hour}:00`,
      avgInterval: Math.round(d.averageMinutes * 10) / 10,
      stdDev: Math.round(d.standardDev * 10) / 10,
      upperBound: Math.round((d.averageMinutes + d.standardDev) * 10) / 10,
      lowerBound: Math.round(Math.max(0, d.averageMinutes - d.standardDev) * 10) / 10
    }));
  }, [analysisData]);

  const formatArrival = (estimatedArrival: string) => {
    if (!estimatedArrival) return 'N/A';
    const arr = new Date(estimatedArrival);
    const now = new Date();
    const diffMs = arr.getTime() - now.getTime();
    const diffMins = Math.round(diffMs / 60000);
    if (diffMins <= 0) return 'Arriving';
    return `${diffMins} min`;
  };

  const getConfidenceText = () => {
    if (!analysisData || analysisData.hourlyData.length === 0) return "Not enough historical data.";

    // Find the current hour's historical standard deviation
    const currentHour = new Date().getHours();
    const hourData = analysisData.hourlyData.find(d => d.hour === currentHour);

    if (!hourData) return "No historical data for this hour.";

    const stdDev = hourData.standardDev;
    if (stdDev < 2) return `High Confidence (Historical variance is ±${Math.round(stdDev)} mins)`;
    if (stdDev < 5) return `Moderate Confidence (Historical variance is ±${Math.round(stdDev)} mins)`;
    return `Low Confidence (Historical variance is ±${Math.round(stdDev)} mins)`;
  };

  return (
    <div className="container">
      <header>
        <h1>Bus Arrival Analysis Dashboard</h1>
        <p>Monitor live arrivals and analyze historical headway distributions.</p>
      </header>

      <div className="controls">
        <div className="control-group">
          <label>Select Bus Stop</label>
          <select value={selectedStop} onChange={e => setSelectedStop(e.target.value)}>
            {loadingStops && <option>Loading...</option>}
            {stops.map(stop => (
              <option key={stop.BusStopCode} value={stop.BusStopCode}>
                {stop.Description} ({stop.BusStopCode})
              </option>
            ))}
          </select>
        </div>

        <div className="control-group">
          <label>Select Route (Bus Number)</label>
          <select value={selectedService} onChange={e => setSelectedService(e.target.value)}>
             {services.length === 0 && <option>No routes available right now</option>}
             {services.map(srv => (
               <option key={srv} value={srv}>{srv}</option>
             ))}
          </select>
        </div>
      </div>

      <div className="dashboard">
        <div className="card">
          <h2>Live Arrivals</h2>
          {liveArrivals ? (
            <div className="arrival-list">
              <div className="arrival-item">
                <span className="arrival-time">{formatArrival(liveArrivals.NextBus?.EstimatedArrival)}</span>
                <span className="arrival-meta">Next Bus • {liveArrivals.NextBus?.Type}</span>
              </div>
              {liveArrivals.NextBus2?.EstimatedArrival && (
                <div className="arrival-item">
                  <span className="arrival-time">{formatArrival(liveArrivals.NextBus2.EstimatedArrival)}</span>
                  <span className="arrival-meta">2nd Bus • {liveArrivals.NextBus2.Type}</span>
                </div>
              )}
              {liveArrivals.NextBus3?.EstimatedArrival && (
                <div className="arrival-item">
                  <span className="arrival-time">{formatArrival(liveArrivals.NextBus3.EstimatedArrival)}</span>
                  <span className="arrival-meta">3rd Bus • {liveArrivals.NextBus3.Type}</span>
                </div>
              )}

              <div className="confidence-indicator">
                <strong>Prediction Confidence</strong>
                <p>{getConfidenceText()}</p>
              </div>
            </div>
          ) : (
             <div className="no-data">No live arrival data for this route at the moment.</div>
          )}
        </div>

        <div className="card" style={{minHeight: '400px'}}>
          <h2>Historical Interval Distribution (Headway)</h2>
          {chartData.length > 0 ? (
             <ResponsiveContainer width="100%" height={350}>
               <LineChart data={chartData} margin={{ top: 20, right: 30, left: 20, bottom: 5 }}>
                 <CartesianGrid strokeDasharray="3 3" />
                 <XAxis dataKey="hour" />
                 <YAxis label={{ value: 'Minutes', angle: -90, position: 'insideLeft' }} />
                 <Tooltip />
                 <Legend />
                 <Line type="monotone" dataKey="avgInterval" name="Average Interval (mins)" stroke="#3182ce" strokeWidth={2} />
                 <Line type="monotone" dataKey="upperBound" name="+1 Std Dev" stroke="#e2e8f0" strokeDasharray="5 5" />
                 <Line type="monotone" dataKey="lowerBound" name="-1 Std Dev" stroke="#e2e8f0" strokeDasharray="5 5" />
               </LineChart>
             </ResponsiveContainer>
          ) : (
            <div className="no-data">
              Not enough historical data recorded for this route at this stop. Data is collected by the background poller over time.
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default App;
