import axios from 'axios';

export enum FieldType {
    INT = 'int',
    TEXT = 'text',
    BOOL = 'bool',
    FLOAT = 'float',
    TIMESTAMP = 'timestamp',
    DATE = 'date',
}

export interface Field {
    uid: string;
    name: string;
    label: string;
    type: FieldType;
    options: object;
    position: number;
}

export function listFields(projectUID: string, tableUID: string) {
    return axios.get<Field[], Field[]>(`/projects/${projectUID}/tables/${tableUID}/fields`);
}

export interface CreateFieldReq {
    fields: {
        name: string;
        label: string;
        type: FieldType;
        options: object;
    }[]
}

export function createFields(projectUID: string, tableUID: string, data: CreateFieldReq) {
    return axios.post<Field, Field>(`/projects/${projectUID}/tables/${tableUID}/fields`, data);
}

export function getField(projectUID: string, tableUID: string, fieldUID: string) {
    return axios.get<Field, Field>(`/projects/${projectUID}/tables/${tableUID}/fields/${fieldUID}`);
}

export interface UpdateFieldReq {
    fields: {
        uid: string;
        name: string;
        label: string;
        type: FieldType;
        options: object;
        position: number;
    }[]
}

export function updateFields(projectUID: string, tableUID: string, data: UpdateFieldReq) {
    return axios.put<void, void>(`/projects/${projectUID}/tables/${tableUID}/fields`, data);
}

export function deleteField(projectUID: string, tableUID: string, fieldUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/tables/${tableUID}/fields/${fieldUID}`);
}
