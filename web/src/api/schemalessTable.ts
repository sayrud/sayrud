import axios from 'axios';

export interface Table {
    uid: string;
    name: string;
    label: string;
    desc: string;
    count: number;
}

export interface ListTablesResp {
    tables: Table[];
    total: number;
}

export function listTables(projectUID: string) {
    return axios.get<ListTablesResp, ListTablesResp>(`/projects/${projectUID}/tables`);
}

export function allTables(projectUID: string) {
    return axios.get<Table[], Table[]>(`/projects/${projectUID}/tables/all`);
}

export function getTable(projectUID: string, tableUID: string) {
    return axios.get<Table, Table>(`/projects/${projectUID}/tables/${tableUID}`);
}

export interface CreateTableReq {
    name: string;
    label: string;
    desc: string;
}

export function createTable(projectUID: string, data: CreateTableReq) {
    return axios.post<Table, Table>(`/projects/${projectUID}/tables`, data);
}

export interface UpdateTableReq {
    label: string;
    desc: string;
}

export function updateTable(projectUID: string, tableUID: string, data: UpdateTableReq) {
    return axios.put<void, void>(`/projects/${projectUID}/tables/${tableUID}`, data);
}

export function deleteTable(projectUID: string, tableUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/tables/${tableUID}`);
}
