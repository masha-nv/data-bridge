import { HttpClient } from '@angular/common/http';
import { computed, inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { EnvironmentService } from './environment.service';

export interface LoginCredentials {
  userId: string;
  password: string;
}

export interface DatabaseConnectionState {
  name: string;
  connected: boolean;
}

export interface EnvironmentConnectionState {
  environment: string;
  databases: DatabaseConnectionState[];
}

export interface SessionResponse {
  connected: boolean;
  userId: string;
  displayName: string;
  mode: string;
  environmentConnections: EnvironmentConnectionState[];
}

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly httpClient = inject(HttpClient);
  private readonly environmentService = inject(EnvironmentService);

  readonly session = signal<SessionResponse | null>(null);
  readonly isAuthenticated = computed(() => this.session()?.connected ?? false);
  readonly displayName = computed(() => this.session()?.displayName ?? '');
  readonly environmentConnections = computed(
    () => this.session()?.environmentConnections ?? [],
  );

  async login(credentials: LoginCredentials): Promise<SessionResponse> {
    const response = await firstValueFrom(
      this.httpClient.post<SessionResponse>(
        this.environmentService.getApiUrl('/api/auth/login'),
        credentials,
      ),
    );

    if (response.connected) {
      this.session.set(response);
    } else {
      this.session.set(null);
    }

    return response;
  }

  logout(): void {
    this.session.set(null);
  }
}