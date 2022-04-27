package biz

type SystemCPUReaderMock struct {
	systemUsage uint64
}

func NewSystemCPUReaderMock(systemUsage uint64) *SystemCPUReaderMock {
	return &SystemCPUReaderMock{
		systemUsage: systemUsage,
	}
}

func (s *SystemCPUReaderMock) ReadUsage() (uint64, error) {
	return s.systemUsage, nil
}
