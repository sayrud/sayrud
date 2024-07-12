import axios from 'axios';

export enum FieldType {
    INT = 'int',
    TEXT = 'text',
    BOOL = 'bool',
    FLOAT = 'float',
    TIMESTAMP = 'timestamp',
    DATE = 'date',
    REFERENCE = 'reference',
    GENERATED = 'generated'
}

export const FieldTypeLabels: Record<string, string> = {
    'int': '整数',
    'text': '文本',
    'bool': '布尔',
    'float': '小数',
    'timestamp': '时间',
    'date': '日期',
    'reference': '引用',
    'generated': '表达式'
}

export interface Field {
    uid: string;
    name: string;
    label: string;
    type: FieldType;
    options: Record<string, any>;
    position: number;
}

export function listFields(projectUID: string, tableUID: string) {
    return axios.get<Field[], Field[]>(`/projects/${projectUID}/tables/${tableUID}/fields`);
}

export interface CreateFieldReq {
    fields: {
        uid?: string;
        name: string;
        label: string;
        type: FieldType;
        options: Record<string, any>;
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
        options: Record<string, any>;
        position: number;
    }[]
}

export function updateFields(projectUID: string, tableUID: string, data: UpdateFieldReq) {
    return axios.put<void, void>(`/projects/${projectUID}/tables/${tableUID}/fields`, data);
}

export function deleteField(projectUID: string, tableUID: string, fieldUID: string) {
    return axios.delete<void, void>(`/projects/${projectUID}/tables/${tableUID}/fields/${fieldUID}`);
}
