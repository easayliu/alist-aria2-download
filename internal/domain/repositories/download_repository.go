package repositories

import (
	"context"
	"github.com/easayliu/alist-aria2-download/internal/domain/entities"
	"github.com/easayliu/alist-aria2-download/internal/domain/valueobjects"
)

// DownloadRepository 下载任务存储库接口
type DownloadRepository interface {
	Create(ctx context.Context, download *entities.Download) error
	GetByID(ctx context.Context, id string) (*entities.Download, error)
	List(ctx context.Context, offset, limit int) ([]*entities.Download, error)
	Update(ctx context.Context, download *entities.Download) error
	Delete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id string, status valueobjects.DownloadStatus) error
}
