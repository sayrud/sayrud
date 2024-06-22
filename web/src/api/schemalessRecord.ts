import axios from 'axios';

export interface Record {
    uid: string;
    data: { [key: number]: any };
}

export interface ListRecordResp {
    records: Record[];
    total: number;
}

export function listRecord(projectUID: string, tableUID: string, params: { page?: number, pageSize?: number }) {
    return axios.get<ListRecordResp, ListRecordResp>(`/projects/${projectUID}/tables/${tableUID}/records`, {params});
}

export function getRecord(projectUID: string, tableUID: string, recordUID: string) {
    return axios.get<Record, Record>(`/projects/${projectUID}/tables/${tableUID}/records/${recordUID}`);
}

export interface CreateRecordReq {
    data: { [key: string]: any }
}

export function createRecord(projectUID: string, tableUID: string, data: CreateRecordReq) {
    return axios.post<Record, Record>(`/projects/${projectUID}/tables/${tableUID}/records`, data);
}

export interface UpdateRecordReq {
    data: { [key: string]: any }
}

export function updateRecord(projectUID: string, tableUID: string, recordUID: string, data: UpdateRecordReq) {
    return axios.put<void, void>(`/projects/${projectUID}/tables/${tableUID}/records/${recordUID}`, data);
}

export function deleteRecord(projectUID: string, tableUID: string, recordUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/tables/${tableUID}/records/${recordUID}`);
}
