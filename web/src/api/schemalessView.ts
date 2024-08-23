import axios from 'axios';
import {Table} from "@/api/schemalessTable.ts";

export interface View {
    uid: string;
    filter: any;
    name: string;
    fieldUIDs: string[];
    order: any;
    table: Table;
}

export interface ListViewsResp {
    views: View[];
    total: number;
}

export function listViews(projectUID: string, params: {
    page?: number,
    pageSize?: number
}) {
    return axios.get<ListViewsResp, ListViewsResp>(`/projects/${projectUID}/views`, {params});
}

export interface CreateViewReq {
    tableUID: string;
    name: string;
    fieldUIDs: string[];
    filter: any;
    order: {
        fieldUID: string;
        orderType: 'asc' | 'desc';
    }[]
}

export function createView(projectUID: string, form: CreateViewReq) {
    return axios.post<View, View>(`/projects/${projectUID}/views`, form);
}

export function getView(projectUID: string, viewUID: string) {
    return axios.get<View, View>(`/projects/${projectUID}/views/${viewUID}`);
}

export interface UpdateViewReq {
    tableUID: string;// Fix lint
    name: string;
    fieldUIDs: string[];
    filter: any;
    order: {
        fieldUID: string;
        orderType: 'asc' | 'desc';
    }[]
}

export function updateView(projectUID: string, viewUID: string, form: UpdateViewReq) {
    return axios.put<void, void>(`/projects/${projectUID}/views/${viewUID}`, form);
}

export function deleteView(projectUID: string, viewUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/views/${viewUID}`);
}

export function queryView(projectUID: string, viewUID: string, params: {
    page?: number,
    pageSize?: number
}) {
    return axios.get<any, any>(`/projects/${projectUID}/views/${viewUID}/query`, {params});
}
