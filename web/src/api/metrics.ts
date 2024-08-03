import axios from 'axios';

export interface MetricsDashboard {
    projectCount: number;
    tableCount: number;
    recordCount: number;
    apiCount: number;
}

export function getMetricsDashboard() {
    return axios.get<MetricsDashboard, MetricsDashboard>('/metrics/dashboard');
}
