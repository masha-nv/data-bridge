import { HttpClient } from '@angular/common/http';
import { computed, inject, Injectable, signal } from '@angular/core';
import { firstValueFrom } from 'rxjs';
import { environment } from '../../environments/environment';
import { ENVIRONMENT } from '../enums';

interface BackendStatusResponse {
  name: string;
  mode: string;
  supportedEnvironments: string[];
  legacyRoutes: string[];
  marxRoutesEnabled: boolean;
}

@Injectable({ providedIn: 'root' })
export class EnvironmentService {
  private readonly httpClient = inject(HttpClient);
  private readonly fallbackEnvironment = 'Dev2';

  readonly loading = signal(false);
  readonly initialized = signal(false);
  readonly errorMessage = signal('');
  readonly appMode = signal('');
  readonly supportedEnvironments = signal<string[]>([this.fallbackEnvironment]);
  readonly legacyRoutes = signal<string[]>([]);
  readonly marxRoutesEnabled = signal(false);

  searchEnvDev = signal(ENVIRONMENT.NONE);
  searchEnvTest = signal(ENVIRONMENT.NONE);
  fromEnv = signal(ENVIRONMENT.NONE);
  toEnv = signal(ENVIRONMENT.NONE);

  readonly defaultEnvironment = computed(
    () => this.supportedEnvironments()[0] ?? this.fallbackEnvironment,
  );

  async ensureLoaded(): Promise<void> {
    if (this.initialized() || this.loading()) {
      return;
    }

    this.loading.set(true);
    this.errorMessage.set('');

    try {
      const status = await firstValueFrom(
        this.httpClient.get<BackendStatusResponse>(this.getApiUrl('/api/status')),
      );

      const supportedEnvironments = [...status.supportedEnvironments].sort();

      this.appMode.set(status.mode);
      this.supportedEnvironments.set(
        supportedEnvironments.length > 0
          ? supportedEnvironments
          : [this.fallbackEnvironment],
      );
      this.legacyRoutes.set(status.legacyRoutes);
      this.marxRoutesEnabled.set(status.marxRoutesEnabled);
      this.initialized.set(true);
    } catch {
      this.errorMessage.set(
        'Unable to load backend status. Start the Electron demo backend and try again.',
      );
    } finally {
      this.loading.set(false);
    }
  }

  getApiUrl(path: string): string {
    if (window.location.protocol === 'file:') {
      return `${environment.apiUrl}${path}`;
    }

    return path;
  }
}
